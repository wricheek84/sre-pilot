import React, { useEffect, useState } from 'react';
import { api } from './api';

function App() {
  const [incidents, setIncidents] = useState([]);
  const [status, setStatus] = useState('CONNECTING');

  useEffect(() => {
    const checkConnection = async () => {
      try {
        const data = await api.getIncidents();
        setIncidents(data);
        setStatus('ONLINE');
      } catch (err) {
        setStatus('OFFLINE');
        console.error("Connectivity Error:", err);
      }
    };
    checkConnection();
  }, []);

  return (
    <div className="min-h-screen bg-background text-foreground p-10 font-sans">
      {/* Vercel-style Header */}
      <header className="mb-12">
        <h1 className="text-[10px] font-mono text-muted uppercase tracking-[0.2em] mb-2">
          SRE-Pilot // Infrastructure Observer
        </h1>
        <div className="flex items-center gap-4">
          <span className="text-4xl font-bold tracking-tighter">GALAXY_VIEW</span>
          <div className={`text-[10px] px-2 py-0.5 rounded border ${
            status === 'ONLINE' ? 'text-new border-new/30 bg-new/5' : 'text-critical border-critical/30 bg-critical/5'
          }`}>
            {status}
          </div>
        </div>
      </header>

      {/* Connectivity Check Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="border border-border bg-surface p-6 rounded-sm">
          <h2 className="text-[10px] font-mono text-muted uppercase mb-4">Active Incidents</h2>
          <p className="text-5xl font-bold tracking-tighter">{incidents.length}</p>
        </div>

        <div className="border border-border bg-surface p-6 rounded-sm">
          <h2 className="text-[10px] font-mono text-muted uppercase mb-4">Latest Incident ID</h2>
          <p className="text-xs font-mono text-muted truncate">
            {incidents.length > 0 ? incidents[0].incident_id : "NO_DATA_SYNCED"}
          </p>
        </div>
      </div>
    </div>
  );
}

export default App;