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
import { cn } from '@/lib/utils';

type Section = 'profile' | 'security' | 'sessions';

const sections: { id: Section; label: string; icon: typeof User }[] = [
  { id: 'profile', label: '个人资料', icon: User },
  { id: 'security', label: '密码安全', icon: KeyRound },
  { id: 'sessions', label: '登录设备', icon: MonitorSmartphone },
];

export default function SettingsPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const profileQuery = useProfile();
  const sessionsQuery = useSessions();
  const updateProfile = useUpdateProfile();
  const changePassword = useChangePassword();
  const revokeSession = useRevokeSession();
  const logout = useLogout();

  const [section, setSection] = useState<Section>('profile');
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
  const sessions = sessionsQuery.data ?? [];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold font-heading text-foreground-950">个人设置</h1>
        <p className="text-sm text-foreground-400 mt-1">管理资料、密码和已登录设备</p>
      </div>

      <div className="flex flex-col lg:flex-row gap-6 items-start">
        {/* Side nav */}
        <nav className="w-full lg:w-48 flex-shrink-0 flex lg:flex-col gap-1 overflow-x-auto">
          {sections.map(item => {
            const Icon = item.icon;
            const active = section === item.id;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setSection(item.id)}
                className={cn(
                  'h-9 px-3 rounded-lg text-sm font-medium inline-flex items-center gap-2 whitespace-nowrap transition-colors',
                  active
                    ? 'bg-primary-500 text-background-50'
                    : 'text-foreground-500 hover:bg-background-100 hover:text-foreground-700',
                )}
              >
                <Icon className="w-4 h-4" />
                {item.label}
              </button>
            );
          })}
          <button
            type="button"
            onClick={handleLogout}
            className="h-9 px-3 rounded-lg text-sm font-medium inline-flex items-center gap-2 text-red-600 hover:bg-red-50 mt-2 lg:mt-4"
          >
            <LogOut className="w-4 h-4" />
            退出登录
          </button>
        </nav>

        {/* Content */}
        <div className="flex-1 w-full min-w-0 max-w-xl">
          {section === 'profile' && (
            <form onSubmit={handleSaveProfile} className="rounded-xl border border-background-200/70 bg-background-50 p-5 space-y-4">
              <h2 className="text-sm font-semibold text-foreground-800 flex items-center gap-2">
                <User className="w-4 h-4 text-primary-500" /> 个人资料
              </h2>
              <div className="flex items-center gap-3 text-sm text-foreground-500">
                <Mail className="w-4 h-4" />
                <span>{user.email}</span>
              </div>
              <label className="block text-xs text-foreground-500">
                用户名
                <input
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                  minLength={2}
                  maxLength={32}
                  required
                  className="mt-1 w-full h-9 px-3 rounded-lg border border-background-200/70 bg-white text-sm"
                />
              </label>
              <label className="block text-xs text-foreground-500">
                简介
                <textarea
                  value={bio}
                  onChange={e => setBio(e.target.value)}
                  maxLength={300}
                  rows={3}
                  className="mt-1 w-full px-3 py-2 rounded-lg border border-background-200/70 bg-white text-sm resize-none"
                />
              </label>
              <button
                type="submit"
                disabled={updateProfile.isPending}
                className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center gap-2"
              >
                {updateProfile.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                保存资料
              </button>
            </form>
          )}

          {section === 'security' && (
            <form onSubmit={handleChangePassword} className="rounded-xl border border-background-200/70 bg-background-50 p-5 space-y-4">
              <h2 className="text-sm font-semibold text-foreground-800 flex items-center gap-2">
                <KeyRound className="w-4 h-4 text-primary-500" /> 修改密码
              </h2>
              <label className="block text-xs text-foreground-500">
                当前密码
                <input
                  type="password"
                  value={currentPassword}
                  onChange={e => setCurrentPassword(e.target.value)}
                  required
                  className="mt-1 w-full h-9 px-3 rounded-lg border border-background-200/70 bg-white text-sm"
                />
              </label>
              <label className="block text-xs text-foreground-500">
                新密码
                <input
                  type="password"
                  value={newPassword}
                  onChange={e => setNewPassword(e.target.value)}
                  required
                  minLength={8}
                  className="mt-1 w-full h-9 px-3 rounded-lg border border-background-200/70 bg-white text-sm"
                />
              </label>
              <label className="inline-flex items-center gap-2 text-sm text-foreground-600">
                <input type="checkbox" checked={revokeOthers} onChange={e => setRevokeOthers(e.target.checked)} />
                修改后注销其他设备
              </label>
              <button
                type="submit"
                disabled={changePassword.isPending}
                className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50"
              >
                {changePassword.isPending ? '更新中…' : '更新密码'}
              </button>
            </form>
          )}

          {section === 'sessions' && (
            <div className="rounded-xl border border-background-200/70 bg-background-50 p-5">
              <h2 className="text-sm font-semibold text-foreground-800 flex items-center gap-2 mb-4">
                <MonitorSmartphone className="w-4 h-4 text-primary-500" /> 登录设备
              </h2>
              {sessionsQuery.isLoading && <LoadingSkeleton count={2} />}
              {sessionsQuery.isError && (
                <ErrorState message="加载会话失败" onRetry={() => sessionsQuery.refetch()} />
              )}
              <ul className="space-y-2">
                {sessions.map(session => (
                  <li
                    key={session.id}
                    className="flex items-center gap-3 p-3 rounded-lg border border-background-200/60 bg-white"
                  >
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-foreground-800 truncate">
                        {session.device || '未知设备'}
                        {session.current && (
                          <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded bg-green-50 text-green-700">当前</span>
                        )}
                      </p>
                      <p className="text-[11px] text-foreground-400 mt-0.5">
                        最近活跃 {session.lastSeenAt ? new Date(session.lastSeenAt).toLocaleString('zh-CN') : '—'}
                      </p>
                    </div>
                    {!session.current && (
                      <button
                        type="button"
                        onClick={() => revokeSession.mutate(session.id, {
                          onSuccess: () => toast('success', '已注销该设备'),
                          onError: err => toast('error', err.message || '操作失败'),
                        })}
                        className="w-8 h-8 rounded-md text-foreground-400 hover:text-red-600 hover:bg-red-50 flex items-center justify-center"
                        aria-label="注销设备"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </li>
                ))}
              </ul>
              {sessions.length === 0 && !sessionsQuery.isLoading && (
                <p className="text-sm text-foreground-400">暂无会话记录</p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
