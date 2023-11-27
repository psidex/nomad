import React, { useRef, useState } from 'react';

import { useSigma } from '@react-sigma/core';
import { Power, PowerOff } from 'lucide-react';

import './css/sidebar.css';

const initialNodeColour = '#5f83cc';
const deadEndNodeColour = '#db4139';

const initialNodeSize = 2;
const maxNodeSize = 10;
const nodeSizeIncrease = (i: number) => i + 0.2;

declare interface NomadSessionConfig {
  workerCooldown: string;
  workerCount: number;
  initialUrls: string[];
  randomCrawl: boolean;
  runtime: string;
  httpClientTimeout: string;
}

enum ButtonStates {
  StartOnly = 1,
  StopOnly,
  ResetOnly,
}

export default function Sidebar() {
  const [waitingToCancel, setWaitingToCancel] = useState<boolean>(false);
  const [wsConnected, setWsConnected] = useState<boolean>(false);
  const [buttonState, setButtonState] = useState<ButtonStates>(ButtonStates.StartOnly);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const ws = useRef<WebSocket>();
  const sigma = useSigma();

  const fileUpload = async (e: React.ChangeEvent<HTMLInputElement>):Promise<void> => {
    if (e.target.files === null) {
      return;
    }
    const file: File = e.target.files[0];
    if (fileInputRef.current !== null) {
      // Reset the selected file so that if the user imports the same file again,
      // the change event will still fire.
      fileInputRef.current.value = '';
    }
    const imported = JSON.parse(await file.text());
    sigma.getGraph().import(imported);
    setButtonState(ButtonStates.ResetOnly);
  };

  const importGraph = () => {
    if (fileInputRef.current !== null) {
      fileInputRef.current.click();
    }
  };

  const exportGraph = () => {
    const exported = sigma.getGraph().export();
    const dataStr = `data:text/json;charset=utf-8,${encodeURIComponent(JSON.stringify(exported))}`;
    const a = document.createElement('a');
    a.setAttribute('href', dataStr);
    a.setAttribute('download', 'nomadgraph.json');
    a.click();
    a.remove();
  };

  const reset = () => {
    sigma.getGraph().clear();
    setButtonState(ButtonStates.StartOnly);
  };

  const cancel = () => {
    if (ws.current !== undefined) {
      setWaitingToCancel(true);
      ws.current.send('cancel');
    }
  };

  const start = (e: any) => {
    e.preventDefault();
    setButtonState(ButtonStates.StopOnly);

    const form = e.target;
    const formData = new FormData(form);
    const formCfg = Object.fromEntries(formData.entries());

    const cfg: NomadSessionConfig = {
      workerCooldown: formCfg.workerCooldown.toString(),
      workerCount: Number(formCfg.workerCount),
      initialUrls: [formCfg.initialUrls.toString()],
      randomCrawl: formCfg.randomCrawl === 'true',
      runtime: formCfg.runtime.toString(),
      httpClientTimeout: formCfg.httpClientTimeout.toString(),
    };

    const sock = new WebSocket('ws://127.0.0.1:8080/ws');
    ws.current = sock;

    sock.onopen = () => {
      setWsConnected(true);
      sock.send(JSON.stringify(cfg));
    };

    sock.onclose = () => {
      setWsConnected(false);
      setWaitingToCancel(false);
      setButtonState(ButtonStates.ResetOnly);
    };

    sock.onmessage = (event) => {
      const msg = JSON.parse(event.data);

      switch (msg.type) {
        case 'node':
          sigma.getGraph().addNode(msg.data.key, {
            x: Math.random(),
            y: Math.random(),
            size: initialNodeSize,
            label: msg.data.attributes.label,
            color: initialNodeColour,
          });
          break;
        case 'nodeupdate':
          sigma.getGraph().updateNode(msg.data.key, (attr) => {
            let newSize = attr.size;
            if (newSize < maxNodeSize) {
              newSize = nodeSizeIncrease(newSize);
            }
            return {
              ...attr,
              ...{
                size: newSize,
              },
            };
          });
          break;
        case 'edge':
          sigma.getGraph().addEdge(msg.data.from, msg.data.to);
          break;
        case 'endcrawl': {
          if (msg.data.deadend === true) {
            sigma.getGraph().updateNode(msg.data.key, (attr) => ({
              ...attr,
              ...{
                color: deadEndNodeColour,
                size: initialNodeSize,
              },
            }));
          }
          break;
        }
        default:
          break;
      }
    };
  };

  return (
    <form className="sidebar" onSubmit={start}>
      <div className="sidebar-status">
        <p>WebSocket Status</p>
        {wsConnected
          ? <Power color="#03c03c" />
          : <PowerOff color="#c23b23" />}
      </div>

      <div className="sidebar-buttons">
        <button type="submit" disabled={buttonState !== ButtonStates.StartOnly}>Start</button>
        <button type="button" onClick={cancel} disabled={buttonState !== ButtonStates.StopOnly || waitingToCancel}>
          {waitingToCancel
            ? '⏳'
            : 'Stop'}
        </button>
        <button type="button" onClick={reset} disabled={buttonState !== ButtonStates.ResetOnly}>Reset</button>
      </div>

      <div className="sidebar-option">
        <label htmlFor="workerCooldownInput">Worker cooldown</label>
        <input name="workerCooldown" id="workerCooldownInput" type="text" defaultValue="2500ms" />
      </div>

      <div className="sidebar-option">
        <label htmlFor="workerCountInput">Worker count</label>
        <input name="workerCount" id="workerCountInput" type="text" defaultValue="3" />
      </div>

      <div className="sidebar-option">
        {/* Support for multiple URLs in future? */}
        <label htmlFor="initialUrlsInput">Initial URL</label>
        <input name="initialUrls" id="initialUrlsInput" type="text" defaultValue="https://www.france.fr/" />
      </div>

      <div className="sidebar-option">
        <label htmlFor="randomCrawlInput">Random crawl</label>
        <input name="randomCrawl" id="randomCrawlInput" type="text" defaultValue="false" />
      </div>

      <div className="sidebar-option">
        <label htmlFor="runtimeInput">Runtime</label>
        <input name="runtime" id="runtimeInput" type="text" defaultValue="10s" />
      </div>

      <div className="sidebar-option">
        <label htmlFor="httpClientTimeoutInput">HTTP client timeout</label>
        <input name="httpClientTimeout" id="httpClientTimeoutInput" type="text" defaultValue="10s" />
      </div>

      <div className="sidebar-buttons">
        <button type="button" onClick={importGraph} disabled={buttonState !== ButtonStates.StartOnly}>Import</button>
        <button type="button" onClick={exportGraph} disabled={buttonState !== ButtonStates.ResetOnly}>Export</button>
      </div>
      <input ref={fileInputRef} type="file" accept="application/json" style={{ display: 'none' }} onChange={fileUpload} />
    </form>
  );
}
