import axios from 'axios';

const API_BASE = "http://localhost:8082";

export const api = {
  getIncidents: async () => {
    const response = await axios.get(`${API_BASE}/incidents`);
    return response.data;
  },

  getIncidentDetail: async (id) => {
    const response = await axios.get(`${API_BASE}/incident/${id}`);
    return response.data;
  },

  getSpatialData: async () => {
    const response = await axios.get(`${API_BASE}/spatial`);
    return response.data;
  }
};