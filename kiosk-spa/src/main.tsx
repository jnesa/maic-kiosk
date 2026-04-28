import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';
import './i18n';
import './index.css';
import { buildRouter } from './router';
import { applyTheme, activeTheme } from './theme';
import { PropertyProvider } from './components/PropertyProvider';
import { propertySlug } from './api/client';

// Apply a sensible default theme up-front so we don't flash an unstyled
// frame while /api/kiosk/v1/<slug>/config is in-flight. The PropertyProvider
// re-applies the property's actual theme as soon as the config resolves.
applyTheme(activeTheme);

// Router is mounted with a basename equal to the property slug so every
// in-app link is automatically scoped to /<slug>/... — components don't
// need to thread the slug into navigate() calls.
const router = buildRouter(`/${propertySlug()}`);

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <PropertyProvider>
      <RouterProvider router={router} />
    </PropertyProvider>
  </React.StrictMode>,
);
