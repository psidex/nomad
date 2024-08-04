import React from 'react';

import { ControlsContainer, ZoomControl, FullScreenControl } from '@react-sigma/core';

import ForceAtlas2Control from './ForceAtlas2Control';
import GraphHoverHighlighter from './GraphHoverHighlighter';
import Sidebar from './Sidebar';

export default function App() {
  return (
    <>
      <GraphHoverHighlighter />
      <ControlsContainer position="top-left">
        <ZoomControl />
        <FullScreenControl />
        <ForceAtlas2Control settings={{ adjustSizes: true, gravity: 0.01, slowDown: 10 }} />
      </ControlsContainer>
      <Sidebar />
    </>
  );
}
