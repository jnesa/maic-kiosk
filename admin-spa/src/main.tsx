import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import './index.css';
import { RootGate } from './components/RootGate';
import { Layout } from './components/Layout';
import { LoginPage } from './features/login/LoginPage';
import { HotelsListPage } from './features/hotels/HotelsListPage';
import { HotelDetailPage } from './features/hotels/HotelDetailPage';
import { AuditLogPage } from './features/audit/AuditLogPage';

// React Query is the data layer. Cache invalidation is explicit per
// mutation (see hotels.ts), so the default stale-time is fine at 0.
const qc = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false },
  },
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={qc}>
      <BrowserRouter basename="/admin">
        <RootGate>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route element={<Layout />}>
              <Route path="/" element={<Navigate to="/hotels" replace />} />
              <Route path="/hotels" element={<HotelsListPage />} />
              <Route path="/hotels/:id" element={<HotelDetailPage />} />
              <Route path="/audit-log" element={<AuditLogPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/hotels" replace />} />
          </Routes>
        </RootGate>
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
);
