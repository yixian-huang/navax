import { useSearchParams } from 'react-router-dom';
import { Archive, CloudCog, Globe2, RefreshCw } from 'lucide-react';
import { cn } from '@/lib/utils';
import ProvidersSection from './components/ProvidersSection';
import UpdateSection from './components/UpdateSection';
import BackupsSection from './components/BackupsSection';
import SubdomainsSection from './components/SubdomainsSection';

type OperationsTab = 'providers' | 'update' | 'backups' | 'subdomains';

const tabs: { id: OperationsTab; label: string; description: string; icon: typeof CloudCog }[] = [
  { id: 'providers', label: '服务配置', description: 'SMTP、存储与 DNS', icon: CloudCog },
  { id: 'update', label: '系统更新', description: '版本与自动更新策略', icon: RefreshCw },
  { id: 'backups', label: '备份恢复', description: '创建、下载与恢复', icon: Archive },
  { id: 'subdomains', label: '短域名审核', description: '1–3 位稀缺子域名与状态管理', icon: Globe2 },
];

export default function AdminOperationsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = searchParams.get('tab');
  const activeTab: OperationsTab = tabs.some(tab => tab.id === requestedTab) ? requestedTab as OperationsTab : 'providers';

  return (
    <div>
      <div className="mb-5"><h1 className="text-xl font-bold font-heading text-foreground-950">运维中心</h1><p className="text-xs text-foreground-400 mt-0.5">管理外部服务、更新、备份恢复和短域名审核</p></div>
      <div className="grid sm:grid-cols-2 xl:grid-cols-4 gap-2 mb-5" role="tablist" aria-label="运维功能">
        {tabs.map(tab => { const Icon = tab.icon; const active = tab.id === activeTab; return <button key={tab.id} role="tab" aria-selected={active} onClick={() => setSearchParams({ tab: tab.id }, { replace: true })} className={cn('text-left rounded-xl border p-3 transition-colors', active ? 'bg-white border-primary-200 shadow-sm' : 'bg-background-50 border-background-200/70 hover:bg-white')}><div className="flex items-center gap-2"><Icon className={cn('w-4 h-4', active ? 'text-primary-500' : 'text-foreground-400')} /><span className="text-sm font-medium text-foreground-700">{tab.label}</span></div><p className="text-[11px] text-foreground-400 mt-1 ml-6">{tab.description}</p></button>; })}
      </div>
      <div role="tabpanel">
        {activeTab === 'providers' ? <ProvidersSection /> : null}
        {activeTab === 'update' ? <UpdateSection /> : null}
        {activeTab === 'backups' ? <BackupsSection /> : null}
        {activeTab === 'subdomains' ? <SubdomainsSection /> : null}
      </div>
    </div>
  );
}
