import React, { useEffect } from 'react';

import { Play, Pause } from 'lucide-react';

import { useWorkerLayoutForceAtlas2 } from '@react-sigma/layout-forceatlas2';

// TODO: Figure out if the settings can be updated once mounted.

// Similar to LayoutForceAtlas2Control but auto starts the layout.
export default function ForceAtlas2Control({ settings }: any) {
  const {
    stop, start, kill, isRunning,
  } = useWorkerLayoutForceAtlas2({ settings });

  const toggle = () => {
    if (isRunning) {
      stop();
    } else {
      start();
    }
  };

  useEffect(() => {
    start();
    return () => {
      // Kill FA2 on unmount
      kill();
    };
  }, [start, kill]);

  return (
    <div className="react-sigma-control">
      <button
        type="button"
        onClick={toggle}
        title={`${isRunning ? 'Pause' : 'Play'} layout animation`}
      >
        {isRunning
          ? <Pause height="1em" width="1em" />
          : <Play height="1em" width="1em" />}
      </button>
    </div>
  );
}
