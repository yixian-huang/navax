import { BrowserRouter, useLocation } from "react-router-dom";
import { AppRoutes } from "./router";
import { I18nextProvider } from "react-i18next";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ToastProvider } from "@/components/base/Toast";
import ErrorBoundary from "@/components/base/ErrorBoundary";
import { Suspense, useEffect } from 'react';
import i18n from "./i18n";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

function ScrollToTop() {
  const { pathname } = useLocation();
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);
  return null;
}

function PageTransitionWrapper({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  return (
    <div key={location.pathname} className="page-enter">
      {children}
    </div>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ToastProvider>
          <ErrorBoundary>
            <BrowserRouter basename={__BASE_PATH__}>
              <ScrollToTop />
              <PageTransitionWrapper>
                <Suspense fallback={<div className="min-h-screen bg-background-50" aria-label="页面加载中" />}>
                  <AppRoutes />
                </Suspense>
              </PageTransitionWrapper>
            </BrowserRouter>
          </ErrorBoundary>
        </ToastProvider>
      </I18nextProvider>
    </QueryClientProvider>
  );
}

export default App;
