# Admin Panel Documentation

## Overview

The admin panel provides a modern, themable web interface for managing the cetus-marketdata-scanner. Built with Alpine.js for reactivity and Go templates for server-side rendering.

## Architecture

### Stack
- **Frontend**: Alpine.js (15kb) + Vanilla CSS + Go templates
- **Backend**: Go HTTP handlers
- **Styling**: CSS custom properties for theming
- **No build step**: Served directly from Go binary

### Components

```
internal/admin/
├── admin.go              # Main handler and routes
├── static/
│   ├── styles.css        # Themable CSS with light/dark modes
│   └── js/
│       ├── api.js        # API client module
│       ├── toast.js      # Toast notifications
│       ├── modal.js      # Modal dialogs
│       └── theme.js      # Theme manager
└── templates/
    ├── base.html         # Base layout with sidebar
    ├── dashboard.html    # Dashboard with stats
    ├── users.html        # User management
    ├── roles.html        # Role management
    ├── groups.html       # Group management
    └── monitoring.html   # System monitoring
```

## Features

### 1. Dashboard
- System statistics (users, API calls, storage)
- Recent activity feed
- User quota visualization
- Real-time updates

### 2. User Management
- Create/edit/delete users
- Assign roles
- View usage and limits
- Enable/disable accounts
- Search and filter

### 3. Role Management
- Create custom roles
- Define capabilities
- Set rate limits and quotas
- Configure permissions
- Visual role cards

### 4. Group Management
- Create/delete groups
- Add/remove members
- Set group leaders
- View group statistics

### 5. Monitoring
- System stats (uptime, memory, connections)
- API performance metrics
- Database statistics
- Throttling statistics
- Recent errors log

## Theming

### CSS Custom Properties

The admin panel uses CSS custom properties for easy theming:

```css
:root {
  /* Colors */
  --color-primary: #3b82f6;
  --color-success: #10b981;
  --color-warning: #f59e0b;
  --color-danger: #ef4444;
  
  /* Backgrounds */
  --bg-primary: #ffffff;
  --bg-secondary: #f8fafc;
  
  /* Text */
  --text-primary: #0f172a;
  --text-secondary: #475569;
  
  /* Borders */
  --border-color: #e2e8f0;
  --border-radius: 8px;
  
  /* Shadows */
  --shadow-sm: 0 1px 2px 0 rgb(0 0 0 / 0.05);
  --shadow-md: 0 4px 6px -1px rgb(0 0 0 / 0.1);
}
```

### Dark Mode

Toggle dark mode with the theme button in the header. Theme preference is saved in localStorage.

```javascript
Theme.set('dark');  // Enable dark mode
Theme.set('light'); // Enable light mode
Theme.toggle();     // Toggle current theme
```

### Custom Themes

Create custom themes by overriding CSS variables:

```css
[data-theme="custom"] {
  --color-primary: #8b5cf6;
  --bg-primary: #1a1a2e;
  /* ... more variables */
}
```

## API Endpoints

### Admin Pages
- `GET /admin/dashboard` - Dashboard page
- `GET /admin/users` - Users management page
- `GET /admin/roles` - Roles management page
- `GET /admin/groups` - Groups management page
- `GET /admin/monitoring` - Monitoring page

### Admin API
- `GET /api/v1/admin/stats` - System statistics
- `GET /api/v1/admin/activities` - Recent activities
- `GET /api/v1/admin/quotas` - User quotas
- `GET /api/v1/admin/monitoring/system` - System metrics
- `GET /api/v1/admin/monitoring/api` - API performance
- `GET /api/v1/admin/monitoring/database` - Database stats
- `GET /api/v1/admin/monitoring/throttle` - Throttling stats
- `GET /api/v1/admin/monitoring/errors` - Recent errors

## Alpine.js Components

### Dashboard Component

```javascript
function dashboard() {
  return {
    stats: { ... },
    activities: [],
    quotas: [],
    
    async init() {
      await this.loadStats();
      await this.loadActivities();
      await this.loadQuotas();
    },
    
    async loadStats() {
      const data = await API.get('/admin/stats');
      this.stats = data;
    }
  };
}
```

### Users Manager Component

```javascript
function usersManager() {
  return {
    users: [],
    filteredUsers: [],
    searchQuery: '',
    roleFilter: '',
    
    filterUsers() {
      this.filteredUsers = this.users.filter(user => {
        const matchesSearch = !this.searchQuery || 
          user.name.toLowerCase().includes(this.searchQuery.toLowerCase());
        const matchesRole = !this.roleFilter || user.roleId === this.roleFilter;
        return matchesSearch && matchesRole;
      });
    }
  };
}
```

## JavaScript Modules

### API Client

```javascript
const API = {
  baseUrl: '/api/v1',
  
  async get(path) {
    const response = await fetch(`${this.baseUrl}${path}`);
    return response.json();
  },
  
  async post(path, data) {
    const response = await fetch(`${this.baseUrl}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    return response.json();
  }
};
```

### Toast Notifications

```javascript
Toast.success('User created successfully');
Toast.error('Failed to load data');
Toast.warning('Quota almost exceeded');
Toast.info('System maintenance scheduled');
```

### Modal Dialogs

```javascript
Modal.show('Create User', '<form>...</form>', [
  { text: 'Cancel', class: 'btn-secondary' },
  { text: 'Save', class: 'btn-primary', onClick: saveUser }
]);

Modal.confirm('Delete User', 'Are you sure?', () => {
  deleteUser();
});
```

## Security

### Authentication
All admin routes require authentication via the `requireAdmin` middleware.

### Authorization
Admin actions check user role and capabilities:
- `system.admin` - Full admin access
- `user.create/read/update/delete` - User management
- `group.create/read/update/delete` - Group management

### Rate Limiting
Admin API endpoints are subject to rate limiting based on user role.

## Customization

### Adding New Pages

1. Create template in `internal/admin/templates/`
2. Add handler in `internal/admin/admin.go`
3. Register route in `RegisterRoutes()`
4. Add navigation link in `base.html`

### Adding New API Endpoints

1. Add handler method in `admin.go`
2. Register route in `RegisterRoutes()`
3. Call from Alpine.js component using `API.get()` or `API.post()`

### Styling Components

Use existing CSS classes:
```html
<button class="btn btn-primary">Save</button>
<input type="text" class="form-input" />
<div class="card">...</div>
<span class="badge badge-success">Active</span>
```

## Performance

### Optimization
- Minimal JavaScript (Alpine.js is 15kb)
- CSS custom properties for fast theme switching
- Embedded assets (no external requests)
- Lazy loading for large data sets

### Caching
- Static assets cached by browser
- API responses can be cached client-side
- Theme preference stored in localStorage

## Troubleshooting

### Common Issues

**Theme not switching**
- Check localStorage is enabled
- Verify `data-theme` attribute is set on `<html>`

**API calls failing**
- Check authentication cookie
- Verify API endpoint paths
- Check browser console for errors

**Alpine.js not working**
- Verify CDN script is loaded
- Check for JavaScript errors
- Ensure `x-data` attributes are correct

### Debug Mode

Enable debug logging:
```javascript
console.log('Dashboard stats:', this.stats);
```

## Future Enhancements

### Planned Features
- Real-time WebSocket updates
- Advanced filtering and search
- Export/import functionality
- Audit log viewer
- Custom dashboard widgets
- Multi-language support

### Integration Ideas
- Grafana dashboards
- Prometheus metrics
- Slack/Email notifications
- Backup management
- User activity analytics

## Support

For issues or questions:
1. Check browser console for errors
2. Review API responses in Network tab
3. Verify user has admin role
4. Check server logs for backend errors

## License

Part of cetus-marketdata-scanner project.
