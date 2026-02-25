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

export function parseAnsi(text: string): AnsiSegment[] {
    const segments: AnsiSegment[] = [];
    // Soporta secuencias ANSI estándar (\x1b[), literales (\\x1b[, \\u001b[)
    // y un caso especial para evitar que se muestren códigos fallidos en el frontend
    const ansiRegex = /[\x1b\u001b]\[([0-9;]*)m|\\x1b\[([0-9;]*)m|\\u001b\[([0-9;]*)m/g;

    let lastIndex = 0;
    let currentFg: string | undefined;
    let currentBg: string | undefined;
    let isBold = false;
    let isItalic = false;
    let isUnderline = false;

    let match;
    while ((match = ansiRegex.exec(text)) !== null) {
        const plainText = text.substring(lastIndex, match.index);
        if (plainText) {
            segments.push({
                text: plainText,
                fg: currentFg,
                bg: currentBg,
                bold: isBold,
                italic: isItalic,
                underline: isUnderline,
            });
        }

        const codes = match[1].split(';').map(Number);
        //const codesStr = match[1] || match[2] || match[3] || '';
        //const codes = codesStr.split(';').map(Number);
        for (let i = 0; i < codes.length; i++) {
            const code = codes[i];
            if (code === 0) {
                currentFg = undefined;
                currentBg = undefined;
                isBold = false;
                isItalic = false;
                isUnderline = false;
            } else if (code === 1) {
                isBold = true;
            } else if (code === 3) {
                isItalic = true;
            } else if (code === 4) {
                isUnderline = true;
            } else if (code >= 30 && code <= 37) {
                // Standard FG
                currentFg = getStandardColor(code - 30);
            } else if (code === 38) {
                // Extended FG
                if (codes[i + 1] === 5) {
                    // 256 colors
                    currentFg = get256Color(codes[i + 2]);
                    i += 2;
                } else if (codes[i + 1] === 2) {
                    // TrueColor
                    currentFg = `rgb(${codes[i + 2]}, ${codes[i + 3]}, ${codes[i + 4]})`;
                    i += 4;
                }
            } else if (code >= 40 && code <= 47) {
                // Standard BG
                currentBg = getStandardColor(code - 40);
            } else if (code === 48) {
                // Extended BG
                if (codes[i + 1] === 5) {
                    currentBg = get256Color(codes[i + 2]);
                    i += 2;
                } else if (codes[i + 1] === 2) {
                    currentBg = `rgb(${codes[i + 2]}, ${codes[i + 3]}, ${codes[i + 4]})`;
                    i += 4;
                }
            } else if (code >= 90 && code <= 97) {
                // Bright FG
                currentFg = getStandardColor(code - 90, true);
            } else if (code >= 100 && code <= 107) {
                // Bright BG
                currentBg = getStandardColor(code - 100, true);
            }
        }
        lastIndex = ansiRegex.lastIndex;
    }

    const remainingText = text.substring(lastIndex);
    if (remainingText) {
        segments.push({
            text: remainingText,
            fg: currentFg,
            bg: currentBg,
            bold: isBold,
            italic: isItalic,
            underline: isUnderline,
        });
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
