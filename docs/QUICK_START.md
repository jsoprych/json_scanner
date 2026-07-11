# Quick Start Guide

## Running the Scanner

### 1. Start the Server

```bash
./run.sh
```

This will:
- Build the binary if needed
- Start the HTTP server on `http://localhost:8080`
- Create a default admin user on first run

### 2. Access the Admin Panel

Open your browser and go to:
```
http://localhost:8080/admin/login
```

### 3. Login with Default Credentials

```
Username: admin
Password: admin
```

⚠️ **IMPORTANT**: Change the default password immediately after first login!

### 4. Change Admin Password

After logging in:
1. Go to **Users** in the sidebar
2. Find the `admin` user
3. Click the **Edit** button (✏️)
4. Enter a new password
5. Click **Save**

## Adding New Users

### Via Admin Panel (Recommended)

1. **Login as admin**
2. Go to **Users** page
3. Click **➕ Create User** button
4. Fill in the form:
   - **User ID**: Unique identifier (e.g., `alice`, `bob`)
   - **Name**: Display name
   - **Password**: Initial password
   - **Role**: Select from available roles:
     - `admin` - Full system access
     - `group_admin` - Can manage groups
     - `user` - Standard user
     - `guest` - Read-only access
   - **Disabled**: Check to disable the account
5. Click **Save**

### Via API

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "alice",
    "name": "Alice Smith",
    "password": "securepassword",
    "role_id": "user"
  }'
```

### Via CLI (JSON File)

Create a `users.json` file:

```json
[
  {
    "id": "alice",
    "name": "Alice Smith",
    "password": "securepassword",
    "role_id": "user"
  },
  {
    "id": "bob",
    "name": "Bob Johnson",
    "password": "anotherpassword",
    "role_id": "group_admin"
  }
]
```

Then use the API to import users.

## User Roles Explained

### admin
- Full system access
- Can manage users, roles, groups
- Can view all data
- Bypasses rate limits
- **Use for**: System administrators

### group_admin
- Can create and manage groups
- Can add/remove group members
- Standard rate limits
- **Use for**: Team leads, department managers

### user
- Can create and manage own studies
- Can save results
- Can join groups
- Standard rate limits
- **Use for**: Regular users, analysts

### guest
- Read-only access to public data
- Cannot create content
- Strict rate limits
- **Use for**: Viewers, read-only access

## Managing Groups

### Create a Group

1. Go to **Groups** page
2. Click **➕ Create Group**
3. Fill in:
   - **Group ID**: Unique identifier (e.g., `traders`, `analysts`)
   - **Name**: Display name
   - **Description**: What the group is for
4. Click **Save**

### Add Members to Group

1. Find the group in the list
2. Click the **👥 Members** button
3. Enter user ID in the input field
4. Click **Add**
5. To make someone a group leader, change their role to `leader`

### Remove Members

1. Open the group's member list
2. Click **Remove** next to the user
3. Confirm the action

## Setting User Limits

### View Current Usage

1. Go to **Users** page
2. Find the user
3. Click the **📊 Limits** button
4. View:
   - API calls (minute/hour/day)
   - Studies created
   - Results saved
   - Replays run

### Override Limits

1. Go to **Users** page
2. Find the user
3. Click **📊 Limits**
4. Modify the limits as needed
5. Click **Save**

Example: Increase API calls per day from 10,000 to 50,000

## Managing Roles

### View Roles

Go to **Roles** page to see all available roles and their capabilities.

### Create Custom Role

1. Click **➕ Create Role**
2. Fill in:
   - **Role ID**: Unique identifier
   - **Name**: Display name
   - **Description**: What this role can do
3. Select **Capabilities**:
   - `user.create`, `user.read`, `user.update`, `user.delete`
   - `group.create`, `group.read`, `group.update`, `group.delete`
   - `study.create`, `study.read`, `study.update`, `study.delete`
   - `result.create`, `result.read`, `result.update`, `result.delete`
   - `system.admin`
4. Set **Limits**:
   - API rate limits
   - Resource quotas
5. Set **Permissions**:
   - Can manage users?
   - Can manage groups?
   - Bypass throttling?
6. Click **Save**

### Edit Role

1. Find the role in the grid
2. Click **✏️ Edit**
3. Modify as needed
4. Click **Save**

⚠️ **Note**: Default roles (`admin`, `user`, `guest`) cannot be deleted.

## Monitoring System

### View Dashboard

Go to **Dashboard** to see:
- Total users
- Active users
- API calls today
- Storage used
- Recent activity
- User quotas

### View Monitoring

Go to **Monitoring** to see:
- System stats (uptime, memory, connections)
- API performance (latency, error rates)
- Database statistics
- Throttling stats
- Recent errors

## Common Tasks

### Reset User Password

1. Go to **Users** page
2. Find the user
3. Click **✏️ Edit**
4. Enter new password
5. Click **Save**

### Disable User Account

1. Go to **Users** page
2. Find the user
3. Click **✏️ Edit**
4. Check **Disabled**
5. Click **Save**

### Delete User

1. Go to **Users** page
2. Find the user
3. Click **🗑️ Delete**
4. Confirm the action

⚠️ **Warning**: This will delete all user data (studies, results, etc.)

### View User Activity

1. Go to **Users** page
2. Find the user
3. Click **📊 Limits**
4. View usage statistics

## Troubleshooting

### Cannot Login

**Problem**: Login fails with "Invalid credentials"

**Solutions**:
1. Check username and password (case-sensitive)
2. Verify user account is not disabled
3. Check if user has the correct role
4. Try resetting the password

### Cannot Access Admin Panel

**Problem**: Redirected to login page

**Solutions**:
1. Verify you're logged in
2. Check if your account has admin role
3. Clear browser cache and cookies
4. Try logging in again

### API Calls Failing

**Problem**: Getting 401 Unauthorized or 429 Too Many Requests

**Solutions**:
1. Check if auth token is valid
2. Verify you haven't exceeded rate limits
3. Check user role permissions
4. Contact admin to increase limits

### User Cannot Access Features

**Problem**: User gets "Forbidden" errors

**Solutions**:
1. Check user's role capabilities
2. Verify resource permissions (owner/group/all)
3. Check if user is in the required group
4. Update role or add ACLs

## Security Best Practices

### Password Management
- ✅ Use strong passwords (12+ characters)
- ✅ Change default passwords immediately
- ✅ Use different passwords for different users
- ❌ Don't share passwords
- ❌ Don't use common passwords

### User Management
- ✅ Follow principle of least privilege
- ✅ Disable unused accounts
- ✅ Review user access regularly
- ❌ Don't give admin access unnecessarily
- ❌ Don't leave default credentials

### Monitoring
- ✅ Check monitoring dashboard regularly
- ✅ Review error logs
- ✅ Monitor rate limit violations
- ❌ Don't ignore security alerts

## Getting Help

### Documentation
- `docs/SYSTEM_OVERVIEW.md` - Complete system documentation
- `docs/ADMIN_PANEL.md` - Admin panel guide
- `docs/API.md` - API reference

### Support
- Check browser console for JavaScript errors
- Check server logs for backend errors
- Review API responses in Network tab
- Verify user permissions and roles

## Next Steps

1. ✅ Start the server with `./run.sh`
2. ✅ Login with admin/admin
3. ✅ Change admin password
4. ✅ Create additional users
5. ✅ Set up groups if needed
6. ✅ Configure roles and permissions
7. ✅ Monitor system usage

---

**Need more help?** See `docs/SYSTEM_OVERVIEW.md` for complete documentation.
