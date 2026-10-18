// Package main provides native Windows system tray integration.
//
// Key behaviors:
// 1. Crisp icon loaded from build/windows/icon.ico (DPI-scaled via SM_CXSMICON).
// 2. Left-click (or double-click) restores and brings the window to focus.
// 3. Right-click displays context menu ("Открыть Послушак" / "Выйти").
// 4. On exit, tray icon is cleanly removed to avoid phantom icons.
// 5. Handles Windows Explorer restarts (TaskbarCreated message).
package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

var isQuitting uint32

func setQuitting() {
	atomic.StoreUint32(&isQuitting, 1)
}

func getQuitting() bool {
	return atomic.LoadUint32(&isQuitting) == 1
}

//go:embed build/windows/icon.ico
var trayIconICO []byte

var (
	modShell32              = windows.NewLazySystemDLL("shell32.dll")
	pShellNotifyIconW       = modShell32.NewProc("Shell_NotifyIconW")
	pExtractIconExW         = modShell32.NewProc("ExtractIconExW")

	modUser32               = windows.NewLazySystemDLL("user32.dll")
	pRegisterClassExW       = modUser32.NewProc("RegisterClassExW")
	pCreateWindowExW        = modUser32.NewProc("CreateWindowExW")
	pDefWindowProcW         = modUser32.NewProc("DefWindowProcW")
	pDestroyWindow          = modUser32.NewProc("DestroyWindow")
	pPostQuitMessage        = modUser32.NewProc("PostQuitMessage")
	pPostMessageW           = modUser32.NewProc("PostMessageW")
	pGetMessageW            = modUser32.NewProc("GetMessageW")
	pTranslateMessage       = modUser32.NewProc("TranslateMessage")
	pDispatchMessageW       = modUser32.NewProc("DispatchMessageW")
	pCreatePopupMenu        = modUser32.NewProc("CreatePopupMenu")
	pAppendMenuW            = modUser32.NewProc("AppendMenuW")
	pDestroyMenu            = modUser32.NewProc("DestroyMenu")
	pTrackPopupMenu         = modUser32.NewProc("TrackPopupMenu")
	pGetCursorPos           = modUser32.NewProc("GetCursorPos")
	pSetForegroundWindow    = modUser32.NewProc("SetForegroundWindow")
	pLoadImageW             = modUser32.NewProc("LoadImageW")
	pLoadIconW              = modUser32.NewProc("LoadIconW")
	pGetSystemMetrics       = modUser32.NewProc("GetSystemMetrics")
	pRegisterWindowMessageW = modUser32.NewProc("RegisterWindowMessageW")

	modKernel32             = windows.NewLazySystemDLL("kernel32.dll")
	pGetModuleHandleW       = modKernel32.NewProc("GetModuleHandleW")
)

const (
	NIM_ADD    = 0x00000000
	NIM_MODIFY = 0x00000001
	NIM_DELETE = 0x00000002

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	WM_DESTROY       = 0x0002
	WM_USER          = 0x0400
	WM_TRAYICON      = WM_USER + 101
	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205
	WM_NULL          = 0x0000

	MF_STRING    = 0x00000000
	MF_SEPARATOR = 0x00000800

	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100
	TPM_NONOTIFY    = 0x0080
	TPM_BOTTOMALIGN = 0x0020
	TPM_LEFTALIGN   = 0x0000

	IMAGE_ICON      = 1
	LR_LOADFROMFILE = 0x00000010
	LR_DEFAULTSIZE  = 0x00000040

	SM_CXSMICON = 49
	SM_CYSMICON = 50

	IDI_APPLICATION = 32512
)

type wndClassEx struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background windows.Handle
	MenuName, ClassName                *uint16
	IconSm                             windows.Handle
}

type notifyIconData struct {
	Size                       uint32
	Wnd                        windows.Handle
	ID, Flags, CallbackMessage uint32
	Icon                       windows.Handle
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout                    uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   windows.GUID
	BalloonIcon                windows.Handle
}

type point struct {
	X, Y int32
}

type winMsg struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

var (
	trayApp      *App
	trayQuitChan chan struct{}
	trayNID      notifyIconData
	trayMu       sync.Mutex
	trayHWND     windows.Handle
	wmTaskbar    uint32
)

func trayWndProc(hWnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_TRAYICON:
		switch lParam {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			// Left click or double-click -> restore and focus the window
			if trayApp != nil {
				trayApp.showMainWindow()
			}
			return 0

		case WM_RBUTTONUP:
			// Right click -> display context menu
			pSetForegroundWindow.Call(uintptr(hWnd))
			var pt point
			pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

			hMenu, _, _ := pCreatePopupMenu.Call()
			if hMenu != 0 {
				titleOpen, _ := windows.UTF16PtrFromString("Открыть Послушак")
				titleQuit, _ := windows.UTF16PtrFromString("Выйти")
				pAppendMenuW.Call(hMenu, MF_STRING, 1, uintptr(unsafe.Pointer(titleOpen)))
				pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
				pAppendMenuW.Call(hMenu, MF_STRING, 2, uintptr(unsafe.Pointer(titleQuit)))

				cmd, _, _ := pTrackPopupMenu.Call(
					hMenu,
					TPM_RETURNCMD|TPM_RIGHTBUTTON|TPM_NONOTIFY|TPM_BOTTOMALIGN|TPM_LEFTALIGN,
					uintptr(pt.X),
					uintptr(pt.Y),
					0,
					uintptr(hWnd),
					0,
				)
				pDestroyMenu.Call(hMenu)
				pPostMessageW.Call(uintptr(hWnd), WM_NULL, 0, 0)

				switch cmd {
				case 1:
					if trayApp != nil {
						trayApp.showMainWindow()
					}
				case 2:
					setQuitting()
					trayRemoveIcon()
					if trayHWND != 0 {
						pPostMessageW.Call(uintptr(trayHWND), WM_DESTROY, 0, 0)
					}
					if trayQuitChan != nil {
						select {
						case <-trayQuitChan:
						default:
							close(trayQuitChan)
						}
					}
					// Failsafe: ensure process terminates completely even if some background goroutines block
					go func() {
						time.Sleep(1500 * time.Millisecond)
						os.Exit(0)
					}()
				}
			}
			return 0
		}

	case wmTaskbar:
		// Explorer restarted -> re-add the tray icon
		trayMu.Lock()
		if trayNID.Wnd != 0 {
			pShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&trayNID)))
		}
		trayMu.Unlock()
		return 0

	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}

	res, _, _ := pDefWindowProcW.Call(uintptr(hWnd), uintptr(message), wParam, lParam)
	return res
}

func (a *App) showMainWindow() {
	if a.ctx != nil {
		wailsRuntime.WindowShow(a.ctx)
		wailsRuntime.WindowUnminimise(a.ctx)
		wailsRuntime.WindowSetAlwaysOnTop(a.ctx, true)
		wailsRuntime.WindowSetAlwaysOnTop(a.ctx, false)
	}
}

func trayRemoveIcon() {
	trayMu.Lock()
	defer trayMu.Unlock()
	if trayNID.Wnd != 0 {
		pShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&trayNID)))
		trayNID.Wnd = 0
	}
}

func loadTrayIcon(instance windows.Handle) windows.Handle {
	// Query current system small icon size (e.g. 16x16 at 100%, 24x24 at 150%)
	cx, _, _ := pGetSystemMetrics.Call(SM_CXSMICON)
	cy, _, _ := pGetSystemMetrics.Call(SM_CYSMICON)
	if cx == 0 {
		cx = 16
	}
	if cy == 0 {
		cy = 16
	}

	// 1. Try embedded icon.ico
	if len(trayIconICO) > 0 {
		tmpPath := filepath.Join(os.TempDir(), "posluchac_tray.ico")
		_ = os.WriteFile(tmpPath, trayIconICO, 0644)
		tmpPathPtr, _ := windows.UTF16PtrFromString(tmpPath)

		h, _, _ := pLoadImageW.Call(
			0,
			uintptr(unsafe.Pointer(tmpPathPtr)),
			IMAGE_ICON,
			cx,
			cy,
			LR_LOADFROMFILE,
		)
		if h != 0 {
			return windows.Handle(h)
		}

		// Fallback to default size
		hDef, _, _ := pLoadImageW.Call(
			0,
			uintptr(unsafe.Pointer(tmpPathPtr)),
			IMAGE_ICON,
			0,
			0,
			LR_LOADFROMFILE|LR_DEFAULTSIZE,
		)
		if hDef != 0 {
			return windows.Handle(hDef)
		}
	}

	// 2. Try extracting from current executable
	if exePath, err := os.Executable(); err == nil {
		exePathPtr, _ := windows.UTF16PtrFromString(exePath)
		var hSmallIcon windows.Handle
		res, _, _ := pExtractIconExW.Call(
			uintptr(unsafe.Pointer(exePathPtr)),
			0,
			0,
			uintptr(unsafe.Pointer(&hSmallIcon)),
			1,
		)
		if res != 0 && hSmallIcon != 0 {
			return hSmallIcon
		}
	}

	// 3. Last fallback: default application icon
	hApp, _, _ := pLoadIconW.Call(0, uintptr(IDI_APPLICATION))
	return windows.Handle(hApp)
}

// runTray starts the native Windows tray message loop in its own thread.
func (a *App) runTray(quit chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	trayApp = a
	trayQuitChan = quit

	instanceHandle, _, _ := pGetModuleHandleW.Call(0)
	instance := windows.Handle(instanceHandle)

	// Register TaskbarCreated message for Explorer restarts
	taskbarStrPtr, _ := windows.UTF16PtrFromString("TaskbarCreated")
	res, _, _ := pRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(taskbarStrPtr)))
	wmTaskbar = uint32(res)

	// Load tray icon
	hIcon := loadTrayIcon(instance)

	classNamePtr, _ := windows.UTF16PtrFromString("PosluchacTrayClass")

	wcex := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   windows.NewCallback(trayWndProc),
		Instance:  instance,
		ClassName: classNamePtr,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		0,
		0,
		0, 0, 0, 0,
		0,
		0,
		uintptr(instance),
		0,
	)
	if hwnd == 0 {
		return
	}
	trayHWND = windows.Handle(hwnd)

	trayMu.Lock()
	trayNID = notifyIconData{
		Size:            uint32(unsafe.Sizeof(notifyIconData{})),
		Wnd:             trayHWND,
		ID:              1,
		Flags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		CallbackMessage: WM_TRAYICON,
		Icon:            hIcon,
	}
	copy(trayNID.Tip[:], windows.StringToUTF16("Послушак"))
	pShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&trayNID)))
	trayMu.Unlock()

	var msg winMsg
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	trayRemoveIcon()
}
