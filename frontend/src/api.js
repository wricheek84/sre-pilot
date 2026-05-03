import axios from 'axios';

// The Go API (Your Observer Port)
const API_BASE = "http://localhost:8082";

// The Qdrant Vector DB (Your Spatial Port)
const QDRANT_BASE = "http://localhost:6333";

export const api = {
  // 1. Get the list of incidents for the sidebar
  getIncidents: async () => {
    const response = await axios.get(`${API_BASE}/incidents`);
    return response.data;
  },

  // 2. Get the "Substance" (ReAct Steps) for a clicked incident
  getIncidentDetail: async (id) => {
    const response = await axios.get(`${API_BASE}/incident/${id}`);
    return response.data;
  },

  // 3. Get the 3D coordinates for the Galaxy View
  getSpatialData: async () => {
    const response = await axios.get(`${QDRANT_BASE}/collections/incidents/points/scroll`, {
      params: { limit: 100, with_payload: true, with_vector: true }
    });
    return response.data.result.points;
  }
};