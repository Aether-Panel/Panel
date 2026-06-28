'use server';
/**
 * @fileOverview A Genkit flow for summarizing server logs and alert messages into concise, easy-to-understand natural language.
 *
 * - summarizeServerAlerts - A function that handles the server alert summarization process.
 * - SummarizeServerAlertsInput - The input type for the summarizeServerAlerts function.
 * - SummarizeServerAlertsOutput - The return type for the summarizeServerAlerts function.
 */

import {ai} from '@/ai/genkit';
import {z} from 'genkit';

const SummarizeServerAlertsInputSchema = z.object({
  logs: z
    .string()
    .describe('Complex server logs or alert messages that need summarization.'),
});
export type SummarizeServerAlertsInput = z.infer<
  typeof SummarizeServerAlertsInputSchema
>;

const SummarizeServerAlertsOutputSchema = z.object({
  summary: z.string().describe('A concise, easy-to-understand summary of the server alerts.'),
});
export type SummarizeServerAlertsOutput = z.infer<
  typeof SummarizeServerAlertsOutputSchema
>;

export async function summarizeServerAlerts(
  input: SummarizeServerAlertsInput
): Promise<SummarizeServerAlertsOutput> {
  return summarizeServerAlertsFlow(input);
}

const summarizeServerAlertsPrompt = ai.definePrompt({
  name: 'summarizeServerAlertsPrompt',
  input: {schema: SummarizeServerAlertsInputSchema},
  output: {schema: SummarizeServerAlertsOutputSchema},
  prompt: `You are an AI assistant tasked with helping administrators quickly understand critical server issues.
Your goal is to take complex server logs or alert messages and summarize them into concise, easy-to-understand natural language.
Focus on extracting the most important information, identifying potential problems, and presenting them clearly.

Server Logs/Alerts:
"""{{{logs}}}"""

Provide a concise summary of the critical issues found in the logs above.`,
});

const summarizeServerAlertsFlow = ai.defineFlow(
  {
    name: 'summarizeServerAlertsFlow',
    inputSchema: SummarizeServerAlertsInputSchema,
    outputSchema: SummarizeServerAlertsOutputSchema,
  },
  async (input) => {
    const {output} = await summarizeServerAlertsPrompt(input);
    return output!;
  }
);
