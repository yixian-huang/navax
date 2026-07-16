import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { KeyRound, Loader2, LogOut, Mail, MonitorSmartphone, Trash2, User } from 'lucide-react';
import {
  useChangePassword,
  useLogout,
  useProfile,
  useRevokeSession,
  useSessions,
  useUpdateProfile,
} from '@/hooks/useQueries';
import { useToast } from '@/components/base/Toast';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';

export default function SettingsPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const profileQuery = useProfile();
  const sessionsQuery = useSessions();
  const updateProfile = useUpdateProfile();
  const changePassword = useChangePassword();
  const revokeSession = useRevokeSession();
  const logout = useLogout();

  const [username, setUsername] = useState('');
  const [bio, setBio] = useState('');
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [revokeOthers, setRevokeOthers] = useState(true);

  useEffect(() => {
    if (!profileQuery.data) return;
    setUsername(profileQuery.data.username);
    setBio(profileQuery.data.bio ?? '');
  }, [profileQuery.data]);

  const handleSaveProfile = (event: React.FormEvent) => {
    event.preventDefault();
    updateProfile.mutate({ username, bio }, {
      onSuccess: () => toast('success', '个人资料已更新'),
      onError: error => toast('error', error.message || '个人资料更新失败'),
    });
  };

  const handleChangePassword = (event: React.FormEvent) => {
    event.preventDefault();
    changePassword.mutate({ currentPassword, newPassword, revokeOtherSessions: revokeOthers }, {
      onSuccess: () => {
        setCurrentPassword('');
        setNewPassword('');
        toast('success', '密码已更新');
      },
      onError: error => toast('error', error.message || '密码更新失败'),
    });
  };

  const handleLogout = () => {
    logout.mutate(undefined, {
      onSuccess: () => navigate('/login', { replace: true }),
      onError: error => toast('error', error.message || '退出失败'),
    });
  };

  if (profileQuery.isLoading) return <LoadingSkeleton count={3} />;
  if (profileQuery.isError || !profileQuery.data) {
    return <ErrorState message={profileQuery.error?.message || '加载用户信息失败'} onRetry={() => profileQuery.refetch()} />;
  }

  const user = profileQuery.data;

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold font-heading text-foreground-950">个人设置</h1>
        <p className="text-sm text-foreground-400 mt-1">管理资料、密码和已登录设备</p>
      </div>

      <div className="max-w-2xl space-y-6">
        <section className="bg-white rounded-xl border border-background-200/70 p-5">
          <h2 className="text-sm font-semibold text-foreground-700 mb-4 flex items-center gap-2">
            <User className="w-4 h-4" />
            个人资料
          </h2>
          <form onSubmit={handleSaveProfile} className="space-y-4">
            <div className="flex items-center gap-4 mb-4">
              <img src={user.avatarUrl} alt={user.username} className="w-16 h-16 rounded-full object-cover object-top bg-background-100" />
              <div>
                <div className="text-sm font-medium text-foreground-900">{user.email}</div>
                <div className="text-xs text-foreground-400">注册于 {new Date(user.createdAt).toLocaleDateString('zh-CN')}</div>
              </div>
            </div>
            <label className="block">
              <span className="block text-xs text-foreground-500 mb-1.5">用户名</span>
              <input
                value={username}
                onChange={event => setUsername(event.target.value)}
                required
                minLength={3}
                maxLength={32}
                className="w-full h-10 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300"
              />
            </label>
            <label className="block">
              <span className="block text-xs text-foreground-500 mb-1.5">个人简介</span>
              <textarea
                value={bio}
                onChange={event => setBio(event.target.value)}
                maxLength={300}
                rows={3}
                className="w-full px-3 py-2 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300 resize-none"
              />
            </label>
            <button disabled={updateProfile.isPending} className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center gap-2">
              {updateProfile.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
              保存资料
            </button>
          </form>
        </section>

        <section className="bg-white rounded-xl border border-background-200/70 p-5">
          <h2 className="text-sm font-semibold text-foreground-700 mb-4 flex items-center gap-2">
            <KeyRound className="w-4 h-4" />
            修改密码
          </h2>
          <form onSubmit={handleChangePassword} className="space-y-3">
            <input
              type="password"
              value={currentPassword}
              onChange={event => setCurrentPassword(event.target.value)}
              placeholder="当前密码"
              required
              className="w-full h-10 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
            />
            <input
              type="password"
              value={newPassword}
              onChange={event => setNewPassword(event.target.value)}
              placeholder="新密码（至少 12 位）"
              required
              minLength={12}
              className="w-full h-10 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
            />
            <label className="flex items-center gap-2 text-xs text-foreground-500">
              <input type="checkbox" checked={revokeOthers} onChange={event => setRevokeOthers(event.target.checked)} />
              同时退出其他设备
            </label>
            <button disabled={changePassword.isPending} className="h-9 px-4 rounded-lg border border-background-200 text-sm text-foreground-700 disabled:opacity-50 inline-flex items-center gap-2">
              {changePassword.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
              更新密码
            </button>
          </form>
        </section>

        <section className="bg-white rounded-xl border border-background-200/70 p-5">
          <h2 className="text-sm font-semibold text-foreground-700 mb-4 flex items-center gap-2">
            <MonitorSmartphone className="w-4 h-4" />
            活动会话
          </h2>
          {sessionsQuery.isLoading ? (
            <div className="text-sm text-foreground-400">加载中...</div>
          ) : sessionsQuery.isError ? (
            <button onClick={() => sessionsQuery.refetch()} className="text-sm text-red-500">加载失败，点击重试</button>
          ) : (
            <div className="divide-y divide-background-100">
              {(sessionsQuery.data ?? []).map(session => (
                <div key={session.id} className="py-3 flex items-center gap-3">
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-foreground-800">
                      {session.device} {session.current && <span className="text-primary-600 text-xs">· 当前会话</span>}
                    </div>
                    <div className="text-xs text-foreground-400">
                      {session.approximateLocation || '未知位置'} · 最近活动 {new Date(session.lastSeenAt).toLocaleString('zh-CN')}
                    </div>
                  </div>
                  {!session.current && (
                    <button
                      onClick={() => revokeSession.mutate(session.id)}
                      disabled={revokeSession.isPending}
                      className="w-8 h-8 rounded-md text-foreground-300 hover:text-red-500 hover:bg-red-50 flex items-center justify-center"
                      aria-label="撤销会话"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              ))}
              {(sessionsQuery.data ?? []).length === 0 && <div className="py-4 text-sm text-foreground-400">暂无活动会话</div>}
            </div>
          )}
        </section>

        <section className="bg-white rounded-xl border border-background-200/70 p-5">
          <h2 className="text-sm font-semibold text-foreground-700 mb-3 flex items-center gap-2"><Mail className="w-4 h-4" />账户信息</h2>
          <div className="text-sm text-foreground-500">{user.email} · {user.role === 'admin' ? '管理员' : '用户'}</div>
        </section>

        <button
          onClick={handleLogout}
          disabled={logout.isPending}
          className="w-full h-10 rounded-lg border border-red-200 text-red-600 text-sm font-medium hover:bg-red-50 disabled:opacity-50 flex items-center justify-center gap-2"
        >
          <LogOut className="w-4 h-4" />
          退出登录
        </button>
      </div>
    </div>
  );
}
