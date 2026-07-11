// API Client Module
const API = {
  baseUrl: '/api/v1',
  
  async request(method, path, data = null) {
    const options = {
      method,
      headers: {
        'Content-Type': 'application/json',
      },
    };
    
    // Add auth token if available
    const token = localStorage.getItem('auth_token');
    if (token) {
      options.headers['Authorization'] = 'Bearer ' + token;
    }
    
    if (data && (method === 'POST' || method === 'PUT' || method === 'PATCH')) {
      options.body = JSON.stringify(data);
    }
    
    const response = await fetch(`${this.baseUrl}${path}`, options);
    
    // Handle unauthorized
    if (response.status === 401) {
      localStorage.removeItem('auth_token');
      localStorage.removeItem('user');
      window.location.href = '/admin/login';
      return;
    }
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Request failed');
    }
    
    if (response.status === 204) {
      return null;
    }
    
    return response.json();
  },
  
  get(path) { return this.request('GET', path); },
  post(path, data) { return this.request('POST', path, data); },
  put(path, data) { return this.request('PUT', path, data); },
  patch(path, data) { return this.request('PATCH', path, data); },
  delete(path) { return this.request('DELETE', path); },
};
