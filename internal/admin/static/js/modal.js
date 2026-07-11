// Modal Module
const Modal = {
  show(title, content, actions = []) {
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay active';
    
    const modal = document.createElement('div');
    modal.className = 'modal';
    
    const header = document.createElement('div');
    header.className = 'modal-header';
    header.innerHTML = `
      <h3 class="modal-title">${title}</h3>
      <button class="modal-close">&times;</button>
    `;
    
    const body = document.createElement('div');
    body.className = 'modal-body';
    body.innerHTML = content;
    
    const footer = document.createElement('div');
    footer.className = 'modal-footer';
    
    actions.forEach(action => {
      const btn = document.createElement('button');
      btn.className = `btn ${action.class || 'btn-secondary'}`;
      btn.textContent = action.text;
      btn.onclick = () => {
        if (action.onClick) action.onClick();
        if (action.close !== false) this.close(overlay);
      };
      footer.appendChild(btn);
    });
    
    header.querySelector('.modal-close').onclick = () => this.close(overlay);
    overlay.onclick = (e) => {
      if (e.target === overlay) this.close(overlay);
    };
    
    modal.appendChild(header);
    modal.appendChild(body);
    if (actions.length > 0) modal.appendChild(footer);
    overlay.appendChild(modal);
    document.body.appendChild(overlay);
    
    return { overlay, body };
  },
  
  close(overlay) {
    overlay.classList.remove('active');
    setTimeout(() => overlay.remove(), 250);
  },
  
  confirm(title, message, onConfirm) {
    this.show(title, `<p>${message}</p>`, [
      { text: 'Cancel', class: 'btn-secondary' },
      { text: 'Confirm', class: 'btn-primary', onClick: onConfirm },
    ]);
  },
};
