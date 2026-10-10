import React from 'react';
import { createRoot } from 'react-dom/client';
import { ReactFlow, Background, Controls } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import './style.css';

const nodes = [{ id: 'start', position: { x: 80, y: 80 }, data: { label: 'Welcome to Orcon' } }];

createRoot(document.getElementById('root')!).render(
  <React.StrictMode><main><header><strong>Orcon</strong><span>Open-source orchestration engine</span></header><section><ReactFlow nodes={nodes} edges={[]} fitView><Background /><Controls /></ReactFlow></section></main></React.StrictMode>
);
