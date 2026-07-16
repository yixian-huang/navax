// ============================================================
// nav.ax ErrorBoundary — graceful crash recovery with retry
// ============================================================

import { Component, type ReactNode } from 'react';
import { AlertTriangle, RotateCw, Home } from 'lucide-react';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    if (import.meta.env.DEV) {
      console.error('[ErrorBoundary]', error, info.componentStack);
    }
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  handleGoHome = () => {
    window.location.href = '/';
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback;

      return (
        <div className="min-h-screen bg-background-50 flex items-center justify-center px-4">
          <div className="flex flex-col items-center text-center max-w-sm">
            <div className="w-16 h-16 rounded-full bg-background-100 flex items-center justify-center mb-5">
              <AlertTriangle className="w-8 h-8 text-primary-400" />
            </div>
            <h2 className="text-lg font-semibold text-foreground-900 mb-2">页面出了点问题</h2>
            <p className="text-sm text-foreground-400 mb-6 leading-relaxed">
              渲染时发生了意外错误。这可能是暂时的，请尝试刷新页面或返回首页。
            </p>
            {import.meta.env.DEV && this.state.error && (
              <pre className="w-full mb-5 p-3 rounded-lg bg-foreground-50 text-xs text-foreground-600 text-left overflow-auto max-h-32 font-mono">
                {this.state.error.message}
              </pre>
            )}
            <div className="flex items-center gap-3">
              <button
                onClick={this.handleGoHome}
                className="inline-flex items-center gap-2 h-9 px-4 rounded-lg border border-background-200 text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
              >
                <Home className="w-4 h-4" />
                返回首页
              </button>
              <button
                onClick={this.handleRetry}
                className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap"
              >
                <RotateCw className="w-4 h-4" />
                重试
              </button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
