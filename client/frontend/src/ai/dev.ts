import { config } from 'dotenv';
config();

import '@/ai/flows/summarize-server-alerts.ts';
import '@/ai/flows/generate-troubleshooting-tips.ts';