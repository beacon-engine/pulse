export interface Toast {
  id: string;
  type: 'error' | 'success' | 'info';
  message: string;
}

class ToastStore {
  toasts = $state<Toast[]>([]);

  push(message: string, type: Toast['type'] = 'info', duration = 4000) {
    const id = Math.random().toString(36).substring(2, 9);
    this.toasts.push({ id, type, message });

    setTimeout(() => {
      this.remove(id);
    }, duration);
  }

  error(message: string) {
    this.push(message, 'error', 5000);
  }

  info(message: string) {
    this.push(message, 'info', 5000);
  }

  success(message: string) {
    this.push(message, 'success', 3000);
  }

  remove(id: string) {
    this.toasts = this.toasts.filter((t) => t.id !== id);
  }
}

export const toast = new ToastStore();
