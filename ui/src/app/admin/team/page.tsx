"use client";

import { useState, useEffect, useCallback } from "react";
import { AdminLayout } from "@/components/layout";
import {
  UserPlus,
  MoreVertical,
  Pencil,
  Key,
  UserMinus,
  UserCheck,
  Trash2,
  X,
  Loader2,
  AlertCircle,
  User,
} from "lucide-react";
import {
  listUsers,
  createUser,
  updateUser,
  deleteUser,
  resetUserPassword,
  getCurrentUser,
  UserSummary,
  UpdateUserRequest,
} from "@/lib/api";

// Helper function for relative time
function formatRelativeTime(dateString?: string): string {
  if (!dateString) return "Never";
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);

  if (diffSec < 60) return "Just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  if (diffDay < 7) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}

// Role Badge Component
function RoleBadge({ role }: { role: "admin" | "member" }) {
  const styles = {
    admin: "bg-purple-100 text-purple-700",
    member: "bg-blue-100 text-blue-700",
  };

  return (
    <span className={`rounded-full px-3 py-1 text-xs font-semibold ${styles[role]}`}>
      {role === "admin" ? "Admin" : "Member"}
    </span>
  );
}

// Status Badge Component
function StatusBadge({ active }: { active: boolean }) {
  return (
    <span
      className={`rounded-full px-3 py-1 text-xs font-semibold ${
        active ? "bg-emerald-100 text-emerald-700" : "bg-gray-100 text-gray-500"
      }`}
    >
      {active ? "Active" : "Inactive"}
    </span>
  );
}

// Dialog Component
function Dialog({
  open,
  onClose,
  title,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
}) {
  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-sercha-ink-slate">{title}</h2>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
          >
            <X className="h-5 w-5" />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

// Add User Dialog
function AddUserDialog({
  open,
  onClose,
  onSuccess,
}: {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [role, setRole] = useState<"admin" | "member">("member");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }

    try {
      setLoading(true);
      await createUser({ email, name, password, role });
      onSuccess();
      onClose();
      // Reset form
      setEmail("");
      setName("");
      setPassword("");
      setConfirmPassword("");
      setRole("member");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create user");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} title="Add User">
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            Email
          </label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
            placeholder="user@example.com"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            Name
          </label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
            placeholder="John Doe"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            Password
          </label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
            placeholder="At least 8 characters"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            Confirm Password
          </label>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
            placeholder="Confirm password"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            Role
          </label>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value as "admin" | "member")}
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
          >
            <option value="member">Member</option>
            <option value="admin">Admin</option>
          </select>
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-sercha-silverline px-4 py-2 text-sm font-medium text-sercha-fog-grey hover:bg-sercha-mist"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90 disabled:opacity-50"
          >
            {loading && <Loader2 className="h-4 w-4 animate-spin" />}
            Add User
          </button>
        </div>
      </form>
    </Dialog>
  );
}

// Edit User Dialog
function EditUserDialog({
  user,
  onClose,
  onSuccess,
}: {
  user: UserSummary | null;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [name, setName] = useState(user?.name || "");
  const [role, setRole] = useState<"admin" | "member">(user?.role || "member");
  const [active, setActive] = useState(user?.active ?? true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (user) {
      setName(user.name);
      setRole(user.role);
      setActive(user.active);
    }
  }, [user]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user) return;
    setError(null);

    const updates: UpdateUserRequest = {};
    if (name !== user.name) updates.name = name;
    if (role !== user.role) updates.role = role;
    if (active !== user.active) updates.active = active;

    if (Object.keys(updates).length === 0) {
      onClose();
      return;
    }

    try {
      setLoading(true);
      await updateUser(user.id, updates);
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update user");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={!!user} onClose={onClose} title="Edit User">
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-fog-grey">
            Email
          </label>
          <input
            type="email"
            value={user?.email || ""}
            disabled
            className="w-full rounded-lg border border-sercha-silverline bg-sercha-snow px-3 py-2 text-sm text-sercha-fog-grey"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            Name
          </label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            Role
          </label>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value as "admin" | "member")}
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
          >
            <option value="member">Member</option>
            <option value="admin">Admin</option>
          </select>
        </div>

        <div className="flex items-center justify-between">
          <label className="text-sm font-medium text-sercha-ink-slate">
            Active
          </label>
          <button
            type="button"
            onClick={() => setActive(!active)}
            className={`relative h-6 w-11 rounded-full transition-colors ${
              active ? "bg-sercha-indigo" : "bg-sercha-silverline"
            }`}
          >
            <span
              className={`absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white transition-transform ${
                active ? "translate-x-5" : "translate-x-0"
              }`}
            />
          </button>
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-sercha-silverline px-4 py-2 text-sm font-medium text-sercha-fog-grey hover:bg-sercha-mist"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90 disabled:opacity-50"
          >
            {loading && <Loader2 className="h-4 w-4 animate-spin" />}
            Save Changes
          </button>
        </div>
      </form>
    </Dialog>
  );
}

// Reset Password Dialog
function ResetPasswordDialog({
  user,
  onClose,
  onSuccess,
}: {
  user: UserSummary | null;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user) return;
    setError(null);

    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }

    try {
      setLoading(true);
      await resetUserPassword(user.id, password);
      onSuccess();
      onClose();
      setPassword("");
      setConfirmPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to reset password");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={!!user} onClose={onClose} title="Reset Password">
      <form onSubmit={handleSubmit} className="space-y-4">
        <p className="text-sm text-sercha-fog-grey">
          Set a new password for <strong>{user?.name}</strong> ({user?.email})
        </p>

        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            New Password
          </label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
            placeholder="At least 8 characters"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
            Confirm Password
          </label>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
            placeholder="Confirm password"
          />
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-sercha-silverline px-4 py-2 text-sm font-medium text-sercha-fog-grey hover:bg-sercha-mist"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90 disabled:opacity-50"
          >
            {loading && <Loader2 className="h-4 w-4 animate-spin" />}
            Reset Password
          </button>
        </div>
      </form>
    </Dialog>
  );
}

// Delete Confirmation Dialog
function DeleteConfirmDialog({
  user,
  onClose,
  onSuccess,
}: {
  user: UserSummary | null;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!user) return;
    setError(null);

    try {
      setLoading(true);
      await deleteUser(user.id);
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete user");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={!!user} onClose={onClose} title="Delete User">
      <div className="space-y-4">
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}

        <div className="rounded-lg bg-red-50 p-4">
          <p className="text-sm text-red-800">
            Are you sure you want to delete <strong>{user?.name}</strong> ({user?.email})?
            This action cannot be undone.
          </p>
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-sercha-silverline px-4 py-2 text-sm font-medium text-sercha-fog-grey hover:bg-sercha-mist"
          >
            Cancel
          </button>
          <button
            onClick={handleDelete}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
          >
            {loading && <Loader2 className="h-4 w-4 animate-spin" />}
            Delete User
          </button>
        </div>
      </div>
    </Dialog>
  );
}

// User Actions Menu
function UserActionsMenu({
  user,
  isCurrentUser,
  onEdit,
  onResetPassword,
  onToggleActive,
  onDelete,
}: {
  user: UserSummary;
  isCurrentUser: boolean;
  onEdit: (user: UserSummary) => void;
  onResetPassword: (user: UserSummary) => void;
  onToggleActive: (user: UserSummary) => void;
  onDelete: (user: UserSummary) => void;
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="rounded-lg p-2 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
      >
        <MoreVertical className="h-4 w-4" />
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute right-0 z-20 mt-1 w-48 rounded-lg border border-sercha-silverline bg-white py-1 shadow-lg">
            <button
              onClick={() => {
                onEdit(user);
                setOpen(false);
              }}
              className="flex w-full items-center gap-2 px-4 py-2 text-sm text-sercha-ink-slate hover:bg-sercha-mist"
            >
              <Pencil className="h-4 w-4" />
              Edit
            </button>
            <button
              onClick={() => {
                onResetPassword(user);
                setOpen(false);
              }}
              className="flex w-full items-center gap-2 px-4 py-2 text-sm text-sercha-ink-slate hover:bg-sercha-mist"
            >
              <Key className="h-4 w-4" />
              Reset Password
            </button>
            <button
              onClick={() => {
                onToggleActive(user);
                setOpen(false);
              }}
              className="flex w-full items-center gap-2 px-4 py-2 text-sm text-sercha-ink-slate hover:bg-sercha-mist"
            >
              {user.active ? (
                <>
                  <UserMinus className="h-4 w-4" />
                  Deactivate
                </>
              ) : (
                <>
                  <UserCheck className="h-4 w-4" />
                  Activate
                </>
              )}
            </button>
            <div className="my-1 border-t border-sercha-mist" />
            <button
              onClick={() => {
                onDelete(user);
                setOpen(false);
              }}
              disabled={isCurrentUser}
              className="flex w-full items-center gap-2 px-4 py-2 text-sm text-red-600 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </button>
          </div>
        </>
      )}
    </div>
  );
}

// Main Team Page
export default function TeamPage() {
  const [users, setUsers] = useState<UserSummary[]>([]);
  const [currentUser, setCurrentUser] = useState<UserSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Dialog states
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [editingUser, setEditingUser] = useState<UserSummary | null>(null);
  const [resettingPasswordFor, setResettingPasswordFor] = useState<UserSummary | null>(null);
  const [deletingUser, setDeletingUser] = useState<UserSummary | null>(null);

  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [usersData, userData] = await Promise.all([
        listUsers(),
        getCurrentUser(),
      ]);
      setUsers(usersData);
      setCurrentUser(userData);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load users");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleToggleActive = async (user: UserSummary) => {
    try {
      await updateUser(user.id, { active: !user.active });
      fetchUsers();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update user");
    }
  };

  return (
    <AdminLayout title="Team" description="Manage team members">
      <div className="space-y-6">
        {/* Header with Add button */}
        <div className="flex items-center justify-between">
          <div />
          <button
            onClick={() => setShowAddDialog(true)}
            className="inline-flex items-center gap-2 rounded-full bg-sercha-indigo px-5 py-2.5 text-sm font-semibold text-white transition-all hover:bg-sercha-indigo/90 hover:shadow-lg"
          >
            <UserPlus size={18} />
            Add User
          </button>
        </div>

        {/* Error message */}
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-4 text-red-600">
            <AlertCircle className="h-5 w-5" />
            {error}
          </div>
        )}

        {/* Loading state */}
        {loading && (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
          </div>
        )}

        {/* Users Table */}
        {!loading && users.length > 0 && (
          <div className="rounded-2xl border-2 border-sercha-silverline bg-white">
            <table className="w-full">
              <thead>
                <tr className="border-b border-sercha-silverline bg-sercha-snow">
                  <th className="px-6 py-4 text-left text-sm font-semibold text-sercha-ink-slate">
                    User
                  </th>
                  <th className="px-6 py-4 text-left text-sm font-semibold text-sercha-ink-slate">
                    Role
                  </th>
                  <th className="px-6 py-4 text-left text-sm font-semibold text-sercha-ink-slate">
                    Status
                  </th>
                  <th className="px-6 py-4 text-left text-sm font-semibold text-sercha-ink-slate">
                    Last Login
                  </th>
                  <th className="px-6 py-4 text-right text-sm font-semibold text-sercha-ink-slate">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr
                    key={user.id}
                    className="border-b border-sercha-mist last:border-0"
                  >
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-sercha-indigo-soft">
                          <User className="h-5 w-5 text-sercha-indigo" />
                        </div>
                        <div>
                          <p className="font-medium text-sercha-ink-slate">
                            {user.name}
                          </p>
                          <p className="text-sm text-sercha-fog-grey">{user.email}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <RoleBadge role={user.role} />
                    </td>
                    <td className="px-6 py-4">
                      <StatusBadge active={user.active} />
                    </td>
                    <td className="px-6 py-4 text-sm text-sercha-fog-grey">
                      {formatRelativeTime(user.last_login_at)}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <UserActionsMenu
                        user={user}
                        isCurrentUser={currentUser?.id === user.id}
                        onEdit={setEditingUser}
                        onResetPassword={setResettingPasswordFor}
                        onToggleActive={handleToggleActive}
                        onDelete={setDeletingUser}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Empty state */}
        {!loading && users.length === 0 && !error && (
          <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-sercha-silverline bg-white py-12">
            <User className="mb-3 h-12 w-12 text-sercha-silverline" />
            <p className="mb-1 text-sm font-medium text-sercha-fog-grey">No users yet</p>
            <p className="mb-4 text-xs text-sercha-silverline">Add your first team member to get started</p>
            <button
              onClick={() => setShowAddDialog(true)}
              className="inline-flex items-center gap-2 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90"
            >
              <UserPlus size={16} />
              Add User
            </button>
          </div>
        )}
      </div>

      {/* Dialogs */}
      <AddUserDialog
        open={showAddDialog}
        onClose={() => setShowAddDialog(false)}
        onSuccess={fetchUsers}
      />
      <EditUserDialog
        user={editingUser}
        onClose={() => setEditingUser(null)}
        onSuccess={fetchUsers}
      />
      <ResetPasswordDialog
        user={resettingPasswordFor}
        onClose={() => setResettingPasswordFor(null)}
        onSuccess={fetchUsers}
      />
      <DeleteConfirmDialog
        user={deletingUser}
        onClose={() => setDeletingUser(null)}
        onSuccess={fetchUsers}
      />
    </AdminLayout>
  );
}
