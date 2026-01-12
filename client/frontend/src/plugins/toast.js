const toast = {
  show(type, message, cb, btnText) {
    const container = document.getElementById('toasts')
    if (!container) return

    const el = document.createElement('div')
    el.classList.add('toast-notification', `toast-${type}`)
    
    // Icono según el tipo
    const iconMap = {
      error: 'hi-exclamation-circle',
      success: 'hi-check-circle',
      warning: 'hi-exclamation-triangle',
      info: 'hi-information-circle'
    }
    
    // Contenido del toast
    el.innerHTML = `
      <div class="toast-content">
        <div class="toast-icon">
          <svg class="toast-icon-svg" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            ${toast.getIconSvg(iconMap[type] || iconMap.info)}
          </svg>
        </div>
        <div class="toast-message">${toast.escapeHtml(message)}</div>
        ${cb ? `<button class="toast-action">${toast.escapeHtml(btnText || 'Acción')}</button>` : ''}
        <button class="toast-close" aria-label="Cerrar">
          <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" class="toast-close-icon">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
          </svg>
        </button>
      </div>
    `
    
    // Event listeners
    const closeBtn = el.querySelector('.toast-close')
    closeBtn.addEventListener('click', () => {
      toast.removeToast(el)
    })
    
    if (cb) {
      const actionBtn = el.querySelector('.toast-action')
      actionBtn.addEventListener('click', () => {
        cb()
        toast.removeToast(el)
      })
    }
    
    container.appendChild(el)
    
    // Animación de entrada
    requestAnimationFrame(() => {
      el.classList.add('toast-enter')
    })
    
    // Auto-remover después de 5 segundos
    let timeout = setTimeout(() => {
      toast.removeToast(el)
    }, 5000)
    
    // Pausar timeout al hacer hover
    el.addEventListener('mouseenter', () => {
      clearTimeout(timeout)
    })
    
    el.addEventListener('mouseleave', () => {
      timeout = setTimeout(() => {
        toast.removeToast(el)
      }, 5000)
    })
  },
  
  removeToast(el) {
    if (!el || !el.parentNode) return
    el.classList.add('toast-exit')
    setTimeout(() => {
      if (el.parentNode) {
        el.parentNode.removeChild(el)
      }
    }, 300)
  },
  
  escapeHtml(text) {
    const div = document.createElement('div')
    div.textContent = text
    return div.innerHTML
  },
  
  getIconSvg(iconType) {
    const icons = {
      'hi-exclamation-circle': '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>',
      'hi-check-circle': '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>',
      'hi-exclamation-triangle': '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>',
      'hi-information-circle': '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>'
    }
    return icons[iconType] || icons['hi-information-circle']
  },
  
  error(message, cb, btnText) {
    this.show('error', message, cb, btnText)
  },
  
  success(message, cb, btnText) {
    this.show('success', message, cb, btnText)
  },
  
  warning(message, cb, btnText) {
    this.show('warning', message, cb, btnText)
  },
  
  info(message, cb, btnText) {
    this.show('info', message, cb, btnText)
  }
}

export default {
  install: (app) => {
    app.config.globalProperties.$toast = toast
    app.provide('toast', toast)
  }
}
