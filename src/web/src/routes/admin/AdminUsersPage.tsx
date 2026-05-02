import { useEffect, useState } from 'react';

interface UserInfo {
  id: string;
  email: string;
  name: string;
  createdAt: string;
  lastLoginAt?: string;
  balance: number;
  used: number;
  agentCount: number;
  taskCount: number;
}

export function AdminUsersPage() {
  const [users, setUsers] = useState<UserInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedUser, setSelectedUser] = useState<UserInfo | null>(null);
  const [quotaInput, setQuotaInput] = useState('');

  const loadUsers = () => {
    fetch('/api/v1/admin/users', { credentials: 'include' })
      .then(res => {
        if (!res.ok) throw new Error('Failed to load users');
        return res.json();
      })
      .then(data => {
        setUsers(data.data);
        setLoading(false);
      })
      .catch(err => {
        setError(err.message);
        setLoading(false);
      });
  };

  useEffect(() => {
    loadUsers();
  }, []);

  const handleUpdateQuota = async () => {
    if (!selectedUser || !quotaInput) return;

    const balance = parseFloat(quotaInput);
    if (isNaN(balance)) {
      alert('Invalid amount');
      return;
    }

    try {
      const res = await fetch(`/api/v1/admin/users/${selectedUser.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ balance })
      });

      if (!res.ok) throw new Error('Failed to update quota');

      setSelectedUser(null);
      setQuotaInput('');
      loadUsers();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleDeleteUser = async (userId: string) => {
    if (!confirm('Are you sure you want to delete this user?')) return;

    try {
      const res = await fetch(`/api/v1/admin/users/${userId}`, {
        method: 'DELETE',
        credentials: 'include'
      });

      if (!res.ok) throw new Error('Failed to delete user');

      loadUsers();
    } catch (err: any) {
      alert(err.message);
    }
  };

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div className="admin-users">
      <h2>User Management</h2>

      <table className="users-table">
        <thead>
          <tr>
            <th>Email</th>
            <th>Name</th>
            <th>Balance</th>
            <th>Used</th>
            <th>Agents</th>
            <th>Tasks</th>
            <th>Last Login</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {users.map(user => (
            <tr key={user.id}>
              <td>{user.email}</td>
              <td>{user.name || '-'}</td>
              <td>${user.balance.toFixed(2)}</td>
              <td>${user.used.toFixed(2)}</td>
              <td>{user.agentCount}</td>
              <td>{user.taskCount}</td>
              <td>{user.lastLoginAt ? new Date(user.lastLoginAt).toLocaleDateString() : 'Never'}</td>
              <td>
                <button onClick={() => { setSelectedUser(user); setQuotaInput(user.balance.toString()); }}>
                  Edit Quota
                </button>
                <button onClick={() => handleDeleteUser(user.id)}>Delete</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {selectedUser && (
        <div className="modal">
          <h3>Edit Quota for {selectedUser.email}</h3>
          <label>
            Balance ($):
            <input
              type="number"
              step="0.01"
              value={quotaInput}
              onChange={e => setQuotaInput(e.target.value)}
            />
          </label>
          <button onClick={handleUpdateQuota}>Save</button>
          <button onClick={() => setSelectedUser(null)}>Cancel</button>
        </div>
      )}
    </div>
  );
}
