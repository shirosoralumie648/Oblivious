import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { RiPencilLine, RiUserForbidLine, RiUserFollowLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';

import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { DrawerForm } from '../../components/shared/DrawerForm';
import { SearchBar } from '../../components/shared/SearchBar';
import { StatusBadge } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { UserDetail, UserUpdateRequest } from '../../types/admin';

type UserForm = {
  role: string;
  planID: string;
  status: 'active' | 'disabled';
};

type UserState = {
  users: UserDetail[];
  loading: boolean;
  error: string | null;
  search: string;
  roleFilter: string;
  statusFilter: string;
  drawerOpen: boolean;
  editingUser: UserDetail | null;
  form: UserForm;
  formLoading: boolean;
  formError: string | null;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; users: UserDetail[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_SEARCH'; value: string }
  | { type: 'SET_ROLE'; value: string }
  | { type: 'SET_STATUS'; value: string }
  | { type: 'OPEN_EDIT'; user: UserDetail }
  | { type: 'CLOSE_DRAWER' }
  | { type: 'FORM_FIELD'; field: keyof UserForm; value: string }
  | { type: 'FORM_START' }
  | { type: 'FORM_ERROR'; error: string }
  | { type: 'FORM_DONE' };

const emptyForm: UserForm = {
  role: 'user',
  planID: '',
  status: 'active',
};

const initialState: UserState = {
  users: [],
  loading: true,
  error: null,
  search: '',
  roleFilter: 'all',
  statusFilter: 'all',
  drawerOpen: false,
  editingUser: null,
  form: emptyForm,
  formLoading: false,
  formError: null,
};

function userToForm(user: UserDetail): UserForm {
  return {
    role: user.role,
    planID: user.planID ?? user.planId ?? '',
    status: user.status,
  };
}

function reducer(state: UserState, action: Action): UserState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, users: action.users };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_SEARCH':
      return { ...state, search: action.value };
    case 'SET_ROLE':
      return { ...state, roleFilter: action.value };
    case 'SET_STATUS':
      return { ...state, statusFilter: action.value };
    case 'OPEN_EDIT':
      return { ...state, drawerOpen: true, editingUser: action.user, form: userToForm(action.user), formError: null };
    case 'CLOSE_DRAWER':
      return { ...state, drawerOpen: false, editingUser: null, formLoading: false, formError: null };
    case 'FORM_FIELD':
      return { ...state, form: { ...state.form, [action.field]: action.value } };
    case 'FORM_START':
      return { ...state, formLoading: true, formError: null };
    case 'FORM_ERROR':
      return { ...state, formLoading: false, formError: action.error };
    case 'FORM_DONE':
      return { ...state, formLoading: false, drawerOpen: false, editingUser: null, formError: null };
    default:
      return state;
  }
}

function usageSummary(user: UserDetail) {
  const stats = user.usageStats;
  const tokens = stats?.totalTokens ?? user.totalTokens ?? 0;
  const calls = stats?.totalAPICalls ?? user.totalApiCalls ?? 0;
  const cost = stats?.totalCost ?? user.totalCost ?? 0;
  return `${tokens.toLocaleString()} tokens / ${calls.toLocaleString()} calls / $${cost.toFixed(2)}`;
}

function lastLogin(value: string | null) {
  if (!value) {
    return 'Never';
  }
  return new Date(value).toLocaleDateString();
}

export function AdminUsersPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadUsers = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const result = await api.listUsers({
        search: state.search,
        role: state.roleFilter === 'all' ? undefined : state.roleFilter,
        status: state.statusFilter === 'all' ? undefined : state.statusFilter,
        limit: 100,
      });
      dispatch({ type: 'LOAD_SUCCESS', users: result.data });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.roleFilter, state.search, state.statusFilter]);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  const handleSubmit = async () => {
    if (!state.editingUser) {
      return;
    }
    const payload: UserUpdateRequest = {
      role: state.form.role,
      planID: state.form.planID.trim() || null,
      status: state.form.status,
    };

    dispatch({ type: 'FORM_START' });
    try {
      await api.updateUser(state.editingUser.id, payload);
      dispatch({ type: 'FORM_DONE' });
      await loadUsers();
    } catch (error) {
      dispatch({ type: 'FORM_ERROR', error: error instanceof Error ? error.message : 'Unable to save user.' });
    }
  };

  const handleStatusAction = async (user: UserDetail) => {
    if (user.status === 'active') {
      await api.disableUser(user.id);
    } else {
      await api.enableUser(user.id);
    }
    await loadUsers();
  };

  const columns: DataTableColumn<UserDetail>[] = [
    { key: 'email', header: 'Email', sortable: true },
    { key: 'name', header: 'Name', render: (user) => user.name || '-' },
    { key: 'role', header: 'Role', render: (user) => <span className="capitalize">{user.role}</span> },
    { key: 'planName', header: 'Plan', render: (user) => user.planName ?? user.planID ?? user.planId ?? '-' },
    { key: 'status', header: 'Status', render: (user) => <StatusBadge status={user.status === 'active' ? 'active' : 'disabled'} /> },
    { key: 'usageStats', header: 'Usage', render: usageSummary },
    { key: 'lastLoginAt', header: 'Last Login', render: (user) => lastLogin(user.lastLoginAt) },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Users</h1>
      </div>

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <SearchBar value={state.search} onChange={(value) => dispatch({ type: 'SET_SEARCH', value })} placeholder="Search users..." />
        <select
          aria-label="Role filter"
          value={state.roleFilter}
          onChange={(event) => dispatch({ type: 'SET_ROLE', value: event.target.value })}
          className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
        >
          <option value="all">All roles</option>
          <option value="admin">Admin</option>
          <option value="user">User</option>
        </select>
        <select
          aria-label="User status filter"
          value={state.statusFilter}
          onChange={(event) => dispatch({ type: 'SET_STATUS', value: event.target.value })}
          className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
        >
          <option value="all">All statuses</option>
          <option value="active">Active</option>
          <option value="disabled">Disabled</option>
        </select>
      </div>

      <DataTable
        columns={columns}
        data={state.users}
        loading={state.loading}
        error={state.error}
        emptyMessage="No users found -- Users will appear here after they register."
        onRetry={loadUsers}
        renderActions={(user) => (
          <div className="flex justify-end gap-1">
            <Button type="button" variant="ghost" size="icon" aria-label={`Edit user ${user.email}`} onClick={() => dispatch({ type: 'OPEN_EDIT', user })}>
              <RiPencilLine className="size-4" aria-hidden="true" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`${user.status === 'active' ? 'Disable' : 'Enable'} user ${user.email}`}
              onClick={() => void handleStatusAction(user)}
            >
              {user.status === 'active' ? <RiUserForbidLine className="size-4" aria-hidden="true" /> : <RiUserFollowLine className="size-4" aria-hidden="true" />}
            </Button>
          </div>
        )}
      />

      <DrawerForm
        open={state.drawerOpen}
        onOpenChange={(open) => {
          if (!open) {
            dispatch({ type: 'CLOSE_DRAWER' });
          }
        }}
        title={state.editingUser ? `Edit User: ${state.editingUser.email}` : 'Edit User'}
        submitLabel="Save User"
        onSubmit={handleSubmit}
        loading={state.formLoading}
        error={state.formError}
      >
        <div className="space-y-2">
          <label htmlFor="user-role" className="text-sm font-medium">Role</label>
          <select
            id="user-role"
            value={state.form.role}
            onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'role', value: event.target.value })}
            className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
          >
            <option value="admin">Admin</option>
            <option value="user">User</option>
          </select>
        </div>
        <div className="space-y-2">
          <label htmlFor="user-plan" className="text-sm font-medium">Plan ID</label>
          <input
            id="user-plan"
            value={state.form.planID}
            onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'planID', value: event.target.value })}
            className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
          />
        </div>
        <div className="space-y-2">
          <label htmlFor="user-status" className="text-sm font-medium">Status</label>
          <select
            id="user-status"
            value={state.form.status}
            onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'status', value: event.target.value })}
            className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
          >
            <option value="active">Active</option>
            <option value="disabled">Disabled</option>
          </select>
        </div>
      </DrawerForm>
    </div>
  );
}
