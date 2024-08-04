import { useState, useEffect } from 'react';

import { useSigma, useRegisterEvents } from '@react-sigma/core';

// https://codesandbox.io/p/sandbox/github/jacomyal/sigma.js/tree/main/examples/use-reducers

const hoveredNodeSmallestSize = 4;

interface GraphInteractionState {
  hoveredNode?: string;
  hoveredNeighbors?: Set<string>;
}

// Highlights a node and it's neighbours when hovered.
export default function GraphHoverHighlighter() {
  const sigma = useSigma();
  const registerEvents = useRegisterEvents();

  const [state, setState] = useState<GraphInteractionState>({});

  useEffect(() => {
    sigma.setSetting('nodeReducer', (node, data) => {
      const res = { ...data };
      if (state.hoveredNeighbors
          && !state.hoveredNeighbors.has(node) && state.hoveredNode !== node) {
        res.label = '';
        res.color = 'white';
      } else if ((state.hoveredNode !== undefined
          && state.hoveredNeighbors
          && state.hoveredNeighbors.has(node)) || state.hoveredNode === node) {
        res.forceLabel = true;
        if (res.size && res.size < hoveredNodeSmallestSize) {
          res.size += hoveredNodeSmallestSize - res.size;
        }
      }
      return res;
    });

    sigma.setSetting('edgeReducer', (edge, data) => {
      const res = { ...data };
      if (state.hoveredNode && !sigma.getGraph().hasExtremity(edge, state.hoveredNode)) {
        res.hidden = true;
      }
      return res;
    });
    // TODO: Not sure if state should be in the dep array?
  }, [sigma, state]);

  useEffect(() => {
    registerEvents({
      enterNode: ({ node }) => {
        setState({
          hoveredNode: node,
          hoveredNeighbors: new Set(sigma.getGraph().neighbors(node)),
        });
      },
      leaveNode: () => {
        setState({
          hoveredNode: undefined,
          hoveredNeighbors: undefined,
        });
      },
    });
  }, [registerEvents, sigma, setState]);

  return null;
}
