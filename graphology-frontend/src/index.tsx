import React from 'react';
import ReactDOM from 'react-dom/client';

import Graph from 'graphology';
import { SigmaContainer } from '@react-sigma/core';

import App from './App';

import '@react-sigma/core/lib/react-sigma.min.css';
import './css/reset.css';

const root = ReactDOM.createRoot(document.getElementById('root')!);
const graph = new Graph();

root.render(
  <React.StrictMode>
    <SigmaContainer style={{ height: '100vh', width: '100vw' }} graph={graph}>
      <App />
    </SigmaContainer>
  </React.StrictMode>,
);
