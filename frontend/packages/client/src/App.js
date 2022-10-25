import 'react-datepicker/dist/react-datepicker.css';
import { DndProvider } from 'react-dnd';
import { HTML5Backend } from 'react-dnd-html5-backend';
import { TouchBackend } from 'react-dnd-touch-backend';
import 'react-draft-wysiwyg/dist/react-draft-wysiwyg.css';
import { ErrorBoundary } from 'react-error-boundary';
import { HashRouter as Router } from 'react-router-dom';
import NotificationModalProvider from 'contexts/NotificationModal';
import { Web3Provider } from 'contexts/Web3';
import { ErrorHandler } from 'components';
import { useMediaQuery } from 'hooks';
import { IS_PRODUCTION } from 'const';
import Hotjar from '@hotjar/browser';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import AppPages from 'pages';
import Error from 'pages/Error';
import './App.sass';

const hotjarVersion = 6;
Hotjar.init(process.env.REACT_APP_HOTJAR_SITE_ID, hotjarVersion);

// create react-query client
const queryClient = new QueryClient();

function App() {
  const notMobile = useMediaQuery();
  return (
    <ErrorBoundary
      fallbackRender={({ error, resetErrorBoundary }) => (
        <Error error={error} resetErrorBoundary={resetErrorBoundary} />
      )}
      onReset={() => (window.location.href = '/')}
    >
      <QueryClientProvider client={queryClient}>
        <Web3Provider network={process.env.REACT_APP_FLOW_ENV}>
          <NotificationModalProvider>
            <Router>
              <ErrorHandler>
                <DndProvider backend={notMobile ? HTML5Backend : TouchBackend}>
                  <AppPages />
                </DndProvider>
              </ErrorHandler>
            </Router>
          </NotificationModalProvider>
        </Web3Provider>
        {!IS_PRODUCTION && <ReactQueryDevtools initialIsOpen={false} />}
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;
