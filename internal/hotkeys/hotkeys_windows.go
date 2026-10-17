//go:build windows

package hotkeys

import (
	"context"
	"runtime"
	"syscall"
	"unsafe"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessage         = user32.NewProc("GetMessageW")
	procPostThreadMessage  = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
)

const (
	VK_MEDIA_NEXT_TRACK = 0xB0
	VK_MEDIA_PREV_TRACK = 0xB1
	VK_MEDIA_PLAY_PAUSE = 0xB3
	WM_HOTKEY           = 0x0312
	WM_QUIT             = 0x0012
)

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type Manager struct {
	cancel context.CancelFunc
}

func Start(ctx context.Context) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	m := &Manager{cancel: cancel}

	threadReady := make(chan uint32)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		tid, _, _ := procGetCurrentThreadId.Call()
		threadId := uint32(tid)
		threadReady <- threadId

		// Register global hotkeys for media keys:
		// ID 1: Play/Pause
		// ID 2: Next Track
		// ID 3: Prev Track
		procRegisterHotKey.Call(0, 1, 0, uintptr(VK_MEDIA_PLAY_PAUSE))
		procRegisterHotKey.Call(0, 2, 0, uintptr(VK_MEDIA_NEXT_TRACK))
		procRegisterHotKey.Call(0, 3, 0, uintptr(VK_MEDIA_PREV_TRACK))

		defer func() {
			procUnregisterHotKey.Call(0, 1)
			procUnregisterHotKey.Call(0, 2)
			procUnregisterHotKey.Call(0, 3)
		}()

		var msg MSG
		for {
			r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			if msg.Message == WM_HOTKEY {
				switch msg.WParam {
				case 1:
					wailsRuntime.EventsEmit(ctx, "media:playpause")
				case 2:
					wailsRuntime.EventsEmit(ctx, "media:next")
				case 3:
					wailsRuntime.EventsEmit(ctx, "media:prev")
				}
			}
		}
	}()

	threadId := <-threadReady

	go func() {
		<-ctx.Done()
		procPostThreadMessage.Call(uintptr(threadId), WM_QUIT, 0, 0)
	}()

	return m
}

func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}
