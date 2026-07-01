'use client';

import React from 'react';

interface AnsiSegment {
    text: string;
    fg?: string;
    bg?: string;
    bold?: boolean;
    italic?: boolean;
    underline?: boolean;
}

interface AnsiState {
    fg?: string;
    bg?: string;
    bold: boolean;
    italic: boolean;
    underline: boolean;
}

function processExtendedColor(codes: number[], i: number): { color?: string, skip: number } {
    if (codes[i + 1] === 5) {
        return { color: get256Color(codes[i + 2]), skip: 2 };
    }
    if (codes[i + 1] === 2) {
        return { color: `rgb(${codes[i + 2]}, ${codes[i + 3]}, ${codes[i + 4]})`, skip: 4 };
    }
    return { skip: 0 };
}

function applyAnsiCode(code: number, codes: number[], i: number, state: AnsiState): number {
    if (code === 0) {
        state.fg = undefined;
        state.bg = undefined;
        state.bold = false;
        state.italic = false;
        state.underline = false;
    } else if (code === 1) state.bold = true;
    else if (code === 3) state.italic = true;
    else if (code === 4) state.underline = true;
    else if (code >= 30 && code <= 37) state.fg = getStandardColor(code - 30);
    else if (code === 38) {
        const { color, skip } = processExtendedColor(codes, i);
        if (color) state.fg = color;
        return skip;
    } else if (code >= 40 && code <= 47) state.bg = getStandardColor(code - 40);
    else if (code === 48) {
        const { color, skip } = processExtendedColor(codes, i);
        if (color) state.bg = color;
        return skip;
    } else if (code >= 90 && code <= 97) state.fg = getStandardColor(code - 90, true);
    else if (code >= 100 && code <= 107) state.bg = getStandardColor(code - 100, true);
    
    return 0;
}

export function parseAnsi(text: string): AnsiSegment[] {
    const segments: AnsiSegment[] = [];
    const ansiRegex = /[\x1b\u001b]\[([0-9;]*)m|\\x1b\[([0-9;]*)m|\\u001b\[([0-9;]*)m/g;

    let lastIndex = 0;
    const state: AnsiState = { bold: false, italic: false, underline: false };
    let match;

    while ((match = ansiRegex.exec(text)) !== null) {
        const plainText = text.substring(lastIndex, match.index);
        if (plainText) {
            segments.push({ text: plainText, ...state });
        }

        const codes = match[1].split(';').map(Number);
        for (let i = 0; i < codes.length; i++) {
            const skip = applyAnsiCode(codes[i], codes, i, state);
            i += skip;
        }
        lastIndex = ansiRegex.lastIndex;
    }

    const remainingText = text.substring(lastIndex);
    if (remainingText) {
        segments.push({ text: remainingText, ...state });
    }

    return segments;
}

function getStandardColor(index: number, bright: boolean = false): string {
    const colors = [
        '#000000', '#cd0000', '#00cd00', '#cdcd00', '#0000ee', '#cd00cd', '#00cdcd', '#e5e5e5',
    ];
    const brightColors = [
        '#7f7f7f', '#ff0000', '#00ff00', '#ffff00', '#5c5cff', '#ff00ff', '#00ffff', '#ffffff',
    ];
    return bright ? brightColors[index] : colors[index];
}

function get256Color(index: number): string {
    if (index < 8) return getStandardColor(index);
    if (index < 16) return getStandardColor(index - 8, true);
    if (index < 232) {
        const r = Math.floor((index - 16) / 36);
        const g = Math.floor(((index - 16) % 36) / 6);
        const b = (index - 16) % 6;
        const rv = r === 0 ? 0 : r * 40 + 55;
        const gv = g === 0 ? 0 : g * 40 + 55;
        const bv = b === 0 ? 0 : b * 40 + 55;
        return `rgb(${rv}, ${gv}, ${bv})`;
    }
    const gray = (index - 232) * 10 + 8;
    return `rgb(${gray}, ${gray}, ${gray})`;
}

export const AnsiText: React.FC<{ text: string }> = ({ text }) => {
    const segments = parseAnsi(text);

    return (
        <>
            {segments.map((segment, i) => (
                <span
                    key={i}
                    style={{
                        color: segment.fg,
                        backgroundColor: segment.bg,
                        fontWeight: segment.bold ? 'bold' : 'normal',
                        fontStyle: segment.italic ? 'italic' : 'normal',
                        textDecoration: segment.underline ? 'underline' : 'none',
                    }}
                >
                    {segment.text}
                </span>
            ))}
        </>
    );
};
