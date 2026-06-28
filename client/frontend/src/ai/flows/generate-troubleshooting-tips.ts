'use server';
/**
 * @fileOverview An AI agent that provides troubleshooting suggestions and potential root causes for server alerts.
 *
 * - generateTroubleshootingTips - A function that handles the generation of troubleshooting tips.
 * - GenerateTroubleshootingTipsInput - The input type for the generateTroubleshootingTips function.
 * - GenerateTroubleshootingTipsOutput - The return type for the generateTroubleshootingTips function.
 */

import {ai} from '@/ai/genkit';
import {z} from 'genkit';

const GenerateTroubleshootingTipsInputSchema = z.object({
  alertSummary: z.string().describe('A concise summary of a server alert or issue.'),
});
export type GenerateTroubleshootingTipsInput = z.infer<typeof GenerateTroubleshootingTipsInputSchema>;

const GenerateTroubleshootingTipsOutputSchema = z.object({
  suggestions: z.array(z.string()).describe('A list of actionable troubleshooting steps.'),
  rootCauses: z.array(z.string()).describe('A list of potential root causes for the server issue.'),
});
export type GenerateTroubleshootingTipsOutput = z.infer<typeof GenerateTroubleshootingTipsOutputSchema>;

export async function generateTroubleshootingTips(
  input: GenerateTroubleshootingTipsInput
): Promise<GenerateTroubleshootingTipsOutput> {
  return generateTroubleshootingTipsFlow(input);
}

const prompt = ai.definePrompt({
  name: 'generateTroubleshootingTipsPrompt',
  input: {schema: GenerateTroubleshootingTipsInputSchema},
  output: {schema: GenerateTroubleshootingTipsOutputSchema},
  prompt: `You are an expert system administrator tasked with providing immediate, actionable troubleshooting suggestions and potential root causes for server issues.

Given the following server alert summary:

Alert Summary: {{{alertSummary}}}

Please provide:
1. A list of actionable troubleshooting suggestions.
2. A list of potential root causes for this issue.`,
});

const generateTroubleshootingTipsFlow = ai.defineFlow(
  {
    name: 'generateTroubleshootingTipsFlow',
    inputSchema: GenerateTroubleshootingTipsInputSchema,
    outputSchema: GenerateTroubleshootingTipsOutputSchema,
  },
  async input => {
    const {output} = await prompt(input);
    return output!;
  }
);
