<script lang="ts" setup>
import { onMounted, onUnmounted } from 'vue'
import { useDialogStore } from '../store/dialog'

const dialog = useDialogStore()

function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (dialog.isConfirmOpen) dialog.handleConfirm(false)
    else if (dialog.isChoiceOpen) dialog.handleChoice(null)
    else if (dialog.isAlertOpen) dialog.handleAlert()
  } else if (e.key === 'Enter') {
    if (dialog.isConfirmOpen) dialog.handleConfirm(true)
    else if (dialog.isAlertOpen) dialog.handleAlert()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<template>
  <!-- Confirm Modal -->
  <div class="modal-backdrop" v-if="dialog.isConfirmOpen" @click.self="dialog.handleConfirm(false)">
    <div class="modal-card">
      <div class="modal-header">
        <div class="modal-title-row">
          <span class="modal-icon" :class="{ danger: dialog.confirmData.danger }">
            {{ dialog.confirmData.danger ? '🗑️' : '❓' }}
          </span>
          <h3>{{ dialog.confirmData.title }}</h3>
        </div>
        <button class="close-btn" @click="dialog.handleConfirm(false)">✕</button>
      </div>

      <div class="modal-body">
        <p class="modal-msg">{{ dialog.confirmData.message }}</p>
        <p class="modal-details" v-if="dialog.confirmData.details">{{ dialog.confirmData.details }}</p>
      </div>

      <div class="modal-footer">
        <button class="btn btn-ghost" @click="dialog.handleConfirm(false)">
          {{ dialog.confirmData.cancelText }}
        </button>
        <button
          class="btn"
          :class="dialog.confirmData.danger ? 'btn-danger' : 'btn-accent'"
          @click="dialog.handleConfirm(true)"
        >
          {{ dialog.confirmData.confirmText }}
        </button>
      </div>
    </div>
  </div>

  <!-- Choice Modal (Multiple Options) -->
  <div class="modal-backdrop" v-if="dialog.isChoiceOpen" @click.self="dialog.handleChoice(null)">
    <div class="modal-card">
      <div class="modal-header">
        <div class="modal-title-row">
          <span class="modal-icon">⚙️</span>
          <h3>{{ dialog.choiceData.title }}</h3>
        </div>
        <button class="close-btn" @click="dialog.handleChoice(null)">✕</button>
      </div>

      <div class="modal-body">
        <p class="modal-msg">{{ dialog.choiceData.message }}</p>
        <p class="modal-details" v-if="dialog.choiceData.details">{{ dialog.choiceData.details }}</p>

        <div class="choice-options">
          <button
            v-for="opt in dialog.choiceData.options"
            :key="opt.value"
            class="choice-btn"
            :class="{ danger: opt.danger, primary: opt.primary }"
            @click="dialog.handleChoice(opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-ghost" @click="dialog.handleChoice(null)">
          {{ dialog.choiceData.cancelText }}
        </button>
      </div>
    </div>
  </div>

  <!-- Alert Modal (Info, Warning, Error) -->
  <div class="modal-backdrop" v-if="dialog.isAlertOpen" @click.self="dialog.handleAlert">
    <div class="modal-card">
      <div class="modal-header">
        <div class="modal-title-row">
          <span class="modal-icon" :class="dialog.alertData.type">
            {{ dialog.alertData.type === 'error' ? '⚠️' : dialog.alertData.type === 'warning' ? '🔔' : dialog.alertData.type === 'success' ? '✅' : 'ℹ️' }}
          </span>
          <h3>{{ dialog.alertData.title }}</h3>
        </div>
        <button class="close-btn" @click="dialog.handleAlert">✕</button>
      </div>

      <div class="modal-body">
        <p class="modal-msg">{{ dialog.alertData.message }}</p>
      </div>

      <div class="modal-footer">
        <button class="btn btn-accent" @click="dialog.handleAlert">
          {{ dialog.alertData.buttonText }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  padding: 20px;
}

.modal-card {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 12px;
  width: 100%;
  max-width: 440px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 20px 48px rgba(0, 0, 0, 0.7);
  animation: modal-appear 0.16s ease-out;
}

@keyframes modal-appear {
  from { opacity: 0; transform: scale(0.95) translateY(6px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 12px;
  border-bottom: 1px solid var(--border);
}

.modal-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.modal-icon {
  font-size: 18px;
}
.modal-icon.danger, .modal-icon.error {
  filter: drop-shadow(0 0 6px rgba(231, 76, 60, 0.5));
}

.modal-title-row h3 {
  font-size: 16px;
  font-weight: 700;
  color: var(--text);
}

.close-btn {
  font-size: 14px;
  color: var(--text2);
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.1s;
}
.close-btn:hover {
  background: var(--surface);
  color: var(--text);
}

.modal-body {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.modal-msg {
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  word-break: break-word;
}

.modal-details {
  font-size: 12px;
  line-height: 1.4;
  color: var(--text2);
  opacity: 0.85;
}

.choice-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 10px;
}

.choice-btn {
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  text-align: left;
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  transition: all 0.12s;
  cursor: pointer;
}
.choice-btn:hover {
  background: var(--surface2);
  border-color: rgba(255, 255, 255, 0.15);
}
.choice-btn.danger {
  color: #ff6b6b;
  border-color: rgba(231, 76, 60, 0.3);
}
.choice-btn.danger:hover {
  background: rgba(231, 76, 60, 0.15);
  border-color: var(--danger);
}
.choice-btn.primary {
  color: var(--accent);
  border-color: rgba(29, 185, 84, 0.3);
}
.choice-btn.primary:hover {
  background: rgba(29, 185, 84, 0.15);
  border-color: var(--accent);
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  background: rgba(0, 0, 0, 0.15);
}

.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}

.btn-ghost {
  color: var(--text2);
}
.btn-ghost:hover {
  color: var(--text);
  background: var(--surface);
}

.btn-accent {
  background: var(--accent);
  color: #000;
}
.btn-accent:hover {
  background: #22d460;
  transform: scale(1.02);
}

.btn-danger {
  background: var(--danger);
  color: #fff;
}
.btn-danger:hover {
  background: #c0392b;
  transform: scale(1.02);
}
</style>
