import { useQuery } from '@tanstack/react-query';
import { authApi } from '@/api/auth';

/**
 * 读取服务端 HttpOnly Cookie 对应的真实会话。
 * 未登录是正常状态：接口返回 authenticated=false、user=null。
 */
export function useAuth() {
  const query = useQuery({
    queryKey: ['auth', 'session'],
    queryFn: async () => (await authApi.getSession()).data,
    staleTime: 5 * 60 * 1000,
  });
  const user = query.data?.user ?? null;

  return {
    ...query,
    user,
    authenticated: query.data?.authenticated === true && user !== null,
    isAdmin: user?.role === 'admin',
  };
}
