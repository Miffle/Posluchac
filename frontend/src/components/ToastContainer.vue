<script lang="ts" setup>
import { useDialogStore } from '../store/dialog'

const dialog = useDialogStore()
</script>

<template>
  <div class="toast-container" v-if="dialog.toasts.length">
    <transition-group name="toast-anim">
      <div
        v-for="t in dialog.toasts"
        :key="t.id"
        class="toast-card"
        :class="t.type"
        @click="dialog.removeToast(t.id)"
        title="Нажмите, чтобы скрыть"
      >
        <span class="toast-icon">
          {{ t.type === 'success' ? '✅' : t.type === 'error' ? '⚠️' : t.type === 'warning' ? '🔔' : 'ℹ️' }}
        </span>
        <span class="toast-text">{{ t.message }}</span>
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.toast-container {
  position: fixed;
  top: 24px;
  right: 28px;
  z-index: 99999;
  display: flex;
  flex-direction: column;
  gap: 10px;
  pointer-events: none;
  max-width: 380px;
}

.toast-card {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 18px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.6);
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s;
  backdrop-filter: blur(8px);
}

.toast-card:hover {
  transform: translateY(-2px);
  filter: brightness(1.1);
}

.toast-card.success {
  border-color: rgba(29, 185, 84, 0.4);
  background: rgba(26, 38, 30, 0.95);
}
.toast-card.success .toast-text {
  color: #a3f7bf;
}

.toast-card.error {
  border-color: rgba(231, 76, 60, 0.4);
  background: rgba(45, 24, 24, 0.95);
}
.toast-card.error .toast-text {
  color: #ff9d9d;
}

.toast-card.warning {
  border-color: rgba(241, 196, 15, 0.4);
  background: rgba(45, 40, 20, 0.95);
}
.toast-card.warning .toast-text {
  color: #f7e08b;
}

.toast-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.toast-text {
  line-height: 1.4;
  word-break: break-word;
}

/* Animations */
.toast-anim-enter-active {
  transition: all 0.25s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}
.toast-anim-leave-active {
  transition: all 0.2s ease-in;
}
.toast-anim-enter-from {
  opacity: 0;
  transform: translateX(30px) scale(0.9);
}
.toast-anim-leave-to {
  opacity: 0;
  transform: translateX(20px) scale(0.95);
}
</style>
