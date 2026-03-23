import axios from "axios";
import { getConfig } from '@/config'

const http = axios.create({
  baseURL: getConfig().API_BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

export default http
