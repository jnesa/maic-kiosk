import { createBrowserRouter, Navigate, Outlet } from 'react-router-dom';
import { WelcomePage } from '@/features/landing/WelcomePage';
import { LookupLastNamePage } from '@/features/landing/LookupLastNamePage';
import { LookupPickGuestPage } from '@/features/landing/LookupPickGuestPage';
import { LookupByIdPage } from '@/features/landing/LookupByIdPage';
import { LookupNotFoundPage } from '@/features/landing/LookupNotFoundPage';
import { CheckinLayout } from '@/features/checkin/CheckinLayout';
import { Step1GuestPage } from '@/features/checkin/Step1GuestPage';
import { Step2FirmPage } from '@/features/checkin/Step2FirmPage';
import { Step3ReviewPage } from '@/features/checkin/Step3ReviewPage';
import { DonePage } from '@/features/done/DonePage';
import { ErrorPage } from '@/features/error/ErrorPage';
import { IdleResetGuard } from '@/components/IdleResetGuard';

const Root = () => (
  <>
    <Outlet />
    <IdleResetGuard />
  </>
);

/**
 * Build the SPA router rooted at the property's URL prefix (e.g. `/smart-moov`).
 * react-router takes care of scoping all relative links automatically —
 * the rest of the SPA can keep using `/checkin/1`, `/lookup/last-name`,
 * etc., and the resulting hrefs become `/smart-moov/checkin/1` and so on.
 */
export const buildRouter = (basename: string) =>
  createBrowserRouter(
    [
      {
        element: <Root />,
        children: [
          { path: '/', element: <WelcomePage /> },
          { path: '/lookup/last-name', element: <LookupLastNamePage /> },
          { path: '/lookup/pick-guest', element: <LookupPickGuestPage /> },
          { path: '/lookup/booking', element: <LookupByIdPage /> },
          { path: '/lookup/not-found', element: <LookupNotFoundPage /> },
          {
            path: '/checkin',
            element: <CheckinLayout />,
            children: [
              { index: true, element: <Navigate to="1" replace /> },
              { path: '1', element: <Step1GuestPage /> },
              { path: '2', element: <Step2FirmPage /> },
              { path: '3', element: <Step3ReviewPage /> },
            ],
          },
          { path: '/done', element: <DonePage /> },
          { path: '/error', element: <ErrorPage /> },
          { path: '*', element: <Navigate to="/" replace /> },
        ],
      },
    ],
    { basename },
  );
