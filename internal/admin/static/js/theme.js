// Theme Manager Module
const Theme = {
  init() {
    const saved = localStorage.getItem('theme') || 'light';
    this.set(saved);
    
    const toggle = document.getElementById('theme-toggle');
    if (toggle) {
      toggle.onclick = () => this.toggle();
    }
  },
  
  set(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
    this.updateIcon(theme);
  },
  
  toggle() {
    const current = document.documentElement.getAttribute('data-theme');
    const next = current === 'dark' ? 'light' : 'dark';
    this.set(next);
  },
  
  updateIcon(theme) {
    const icon = document.querySelector('#theme-toggle .icon');
    if (icon) {
      icon.textContent = theme === 'dark' ? '☀️' : '🌙';
    }
  },
};
