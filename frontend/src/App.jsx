import React, { useEffect, useState } from 'react';
import { api } from './api';
import { X, Terminal, Cpu, Activity, Shield, Clock } from 'lucide-react';
import { Canvas } from '@react-three/fiber';
import { Galaxy } from './components/Galaxy';

function App() {
  const [incidents, setIncidents] = useState([]);
  const [spatialData, setSpatialData] = useState([]);
  const [selectedId, setSelectedId] = useState(null);
  const [selectedIncident, setSelectedIncident] = useState(null);
  const [status, setStatus] = useState('CONNECTING');

  useEffect(() => {
    const loadData = async () => {
      try {
        const [incData, spatial] = await Promise.all([
          api.getIncidents(),
          api.getSpatialData()
        ]);
        setIncidents(incData);
        setSpatialData(spatial);
        setStatus('ONLINE');
      } catch (err) {
        setStatus('OFFLINE');
      }
    };
    loadData();
    const interval = setInterval(loadData, 5000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (!selectedId) return;
    api.getIncidentDetail(selectedId)
      .then(setSelectedIncident)
      .catch(() => console.error("Detail fetch failed"));
  }, [selectedId]);

  return (
    <div className="h-screen w-screen bg-background text-foreground flex overflow-hidden font-sans select-none">
      
      <aside className="w-80 border-r border-border bg-surface flex flex-col z-20 shrink-0">
        <div className="p-4 border-b border-border flex items-center justify-between">
          <h2 className="text-[10px] font-mono font-bold tracking-widest text-muted">INCIDENTS_FEED</h2>
          <span className={`text-[10px] font-mono px-2 py-0.5 rounded border uppercase ${
            status === 'ONLINE' ? 'text-new bg-new/10 border-new/20' : 'text-critical bg-critical/10 border-critical/20'
          }`}>
            {status}
          </span>
        </div>

        <div className="flex-1 overflow-y-auto">
          {incidents.map((inc) => (
            <div 
              key={inc.id}
              onClick={() => setSelectedId(inc.incident_id)}
              className={`group flex items-center p-4 border-b border-border cursor-pointer transition-colors hover:bg-white/[0.02] ${selectedId === inc.incident_id ? 'bg-white/[0.04]' : ''}`}
            >
              <div className={`w-1 h-10 mr-4 rounded-full ${
                inc.action_taken === 'AGENT_FIXED' ? 'bg-new shadow-[0_0_8px_rgba(16,185,129,0.5)]' : 'bg-critical'
              }`} />
              <div className="flex-1 min-w-0">
                <div className="flex justify-between items-start mb-1">
                  <span className="text-[10px] font-mono text-muted truncate uppercase tracking-tighter">
                    {inc.incident_id.slice(0, 8)}
                  </span>
                  <span className="text-[10px] font-mono text-muted">{inc.mttr}ms</span>
                </div>
                <h3 className="text-sm font-medium truncate leading-tight">{inc.log_line}</h3>
              </div>
            </div>
          ))}
        </div>
      </aside>

      <main className="flex-1 flex relative bg-[#020202] overflow-hidden">
        
        <div className="flex-1 relative min-w-0">
          <div className="absolute top-8 left-8 z-10 pointer-events-none">
            <h1 className="text-2xl font-mono font-bold text-muted">Grid</h1>
            <p className="text-[10px] font-mono text-muted uppercase tracking-[0.3em] mt-1">
              Semantic Vector Space / Cluster_01
            </p>
          </div>

          <Canvas gl={{ antialias: true, alpha: true }} dpr={[1, 1.5]}>
            <color attach="background" args={['#000000']} />
            <Galaxy points={spatialData} selectedId={selectedId}/>
          </Canvas>
        </div>

        {selectedIncident && (
          <div className="w-[450px] h-full bg-surface/95 border-l border-border z-40 flex flex-col shadow-2xl shrink-0">
            <div className="p-6 border-b border-border flex items-center justify-between">
              <div>
                <h2 className="text-xs font-mono font-bold text-muted uppercase tracking-widest">Incident_Detail</h2>
                <p className="text-[10px] font-mono text-critical mt-1">{selectedIncident.incident_id}</p>
              </div>
              <button onClick={() => { setSelectedIncident(null); setSelectedId(null); }} className="p-2 hover:bg-white/10 rounded-full"><X size={18} /></button>
            </div>

            <div className="flex-1 overflow-y-auto p-6 space-y-6">
              <section className="grid grid-cols-3 gap-3">
                <div className="p-3 bg-white/[0.03] border border-border rounded-lg text-center">
                  <div className="flex justify-center text-muted mb-1"><Activity size={12} /></div>
                  <p className="text-sm font-bold font-mono">{selectedIncident.blast_radius}</p>
                  <span className="text-[8px] uppercase text-muted font-mono">Radius</span>
                </div>
                <div className="p-3 bg-white/[0.03] border border-border rounded-lg text-center">
                  <div className="flex justify-center text-muted mb-1"><Shield size={12} /></div>
                  <p className="text-sm font-bold font-mono">{(selectedIncident.trust_score * 100).toFixed(0)}%</p>
                  <span className="text-[8px] uppercase text-muted font-mono">Trust</span>
                </div>
                <div className="p-3 bg-white/[0.03] border border-border rounded-lg text-center">
                  <div className="flex justify-center text-muted mb-1"><Clock size={12} /></div>
                  <p className="text-sm font-bold font-mono">{selectedIncident.mttr}ms</p>
                  <span className="text-[8px] uppercase text-muted font-mono">MTTR</span>
                </div>
              </section>

              <section>
                <div className="flex items-center gap-2 text-muted mb-4 border-b border-border pb-2">
                  <Terminal size={14} />
                  <h3 className="text-[10px] uppercase font-mono font-bold tracking-wider">ReAct_Trace</h3>
                </div>
                <div className="space-y-3">
                  {(() => {
                    try {
                      const steps = JSON.parse(selectedIncident.steps || '[]');
                      return steps.map((step, idx) => (
                        <div key={idx} className="relative pl-6 border-l border-white/10 pb-4 last:pb-0">
                          <div className="absolute -left-[4.5px] top-1.5 w-2 h-2 rounded-full bg-critical border border-black" />
                          <p className="text-[11px] font-mono text-foreground/70 leading-relaxed bg-white/[0.02] p-2 rounded">
                            <span className="text-critical/80 mr-2">[{step.StepType}]</span>
                            {step.Content}
                          </p>
                        </div>
                      ));
                    } catch (e) {
                      return <p className="text-[10px] font-mono text-muted">Awaiting final agent report...</p>;
                    }
                  })()}
                </div>
              </section>

              <section className={`p-4 rounded-lg border transition-colors duration-500 ${
                selectedIncident.action_taken === 'AGENT_FIXED' 
                  ? 'bg-new/5 border-new/20' 
                  : 'bg-critical/5 border-critical/20'
              }`}>
                <div className={`flex items-center gap-2 mb-1 ${
                  selectedIncident.action_taken === 'AGENT_FIXED' ? 'text-new' : 'text-critical'
                }`}>
                  <Cpu size={14} />
                  <h3 className="text-[10px] uppercase font-mono font-bold">Resolution</h3>
                </div>
                <p className={`text-xs font-medium italic leading-relaxed ${
                  selectedIncident.action_taken === 'AGENT_FIXED' ? 'text-new/90' : 'text-critical/80'
                }`}>
                  {selectedIncident.action_taken === 'AGENT_FIXED' 
                    ? 'Root cause isolated via ReAct loop. Automated remediation successful.' 
                    : 'Event logged. No critical failure detected requiring intervention.'}
                </p>
              </section>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}

export default App;