// Reading and writing Minecraft's server.properties without losing anything: comments, order,
// blank lines and keys the panel does not know are kept as they are, and only the keys that
// changed are rewritten. Values follow java.util.Properties escaping, which is what Minecraft
// writes (e.g. "level-type=minecraft\:normal", "motd=§aHello").

export type PropertiesLine =
  | { kind: 'other'; raw: string }
  | { kind: 'prop'; raw: string; key: string; value: string };

export type ParsedProperties = {
  lines: PropertiesLine[];
  values: Record<string, string>;
};

function unescape(text: string): string {
  let out = '';
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (ch !== '\\' || i === text.length - 1) {
      out += ch;
      continue;
    }
    const next = text[++i];
    switch (next) {
      case 'n': out += '\n'; break;
      case 't': out += '\t'; break;
      case 'r': out += '\r'; break;
      case 'f': out += '\f'; break;
      case 'u': {
        const hex = text.slice(i + 1, i + 5);
        if (/^[0-9a-fA-F]{4}$/.test(hex)) {
          out += String.fromCharCode(parseInt(hex, 16));
          i += 4;
        } else {
          out += 'u';
        }
        break;
      }
      default: out += next;
    }
  }
  return out;
}

/** Escapes a value the way Properties.store does, so every Minecraft version reads it back. */
export function escapeValue(value: string): string {
  let out = '';
  for (let i = 0; i < value.length; i++) {
    const ch = value[i];
    const code = ch.charCodeAt(0);
    if (ch === '\\') out += '\\\\';
    else if (ch === '\n') out += '\\n';
    else if (ch === '\r') out += '\\r';
    else if (ch === '\t') out += '\\t';
    else if (ch === '\f') out += '\\f';
    else if (ch === ' ' && i === 0) out += '\\ ';
    else if (ch === '=' || ch === ':' || ch === '#' || ch === '!') out += '\\' + ch;
    else if (code < 0x20 || code > 0x7e) out += '\\u' + code.toString(16).toUpperCase().padStart(4, '0');
    else out += ch;
  }
  return out;
}

/** Splits "key=value", "key: value" or "key value" at the first unescaped separator. */
function splitLine(line: string): { key: string; value: string } {
  let i = 0;
  let key = '';
  for (; i < line.length; i++) {
    const ch = line[i];
    if (ch === '\\' && i + 1 < line.length) {
      key += ch + line[++i];
      continue;
    }
    if (ch === '=' || ch === ':' || ch === ' ' || ch === '\t' || ch === '\f') break;
    key += ch;
  }
  // whitespace, then at most one '=' or ':', then whitespace
  while (i < line.length && /[ \t\f]/.test(line[i])) i++;
  if (i < line.length && (line[i] === '=' || line[i] === ':')) i++;
  while (i < line.length && /[ \t\f]/.test(line[i])) i++;
  return { key: unescape(key), value: unescape(line.slice(i)) };
}

/** A line continues on the next one when it ends with an odd number of backslashes. */
function continues(line: string): boolean {
  let count = 0;
  for (let i = line.length - 1; i >= 0 && line[i] === '\\'; i--) count++;
  return count % 2 === 1;
}

export function parseProperties(text: string): ParsedProperties {
  const rawLines = text.replace(/\r\n?/g, '\n').split('\n');
  // a trailing newline leaves an empty last entry that is not a real line
  if (rawLines.length > 0 && rawLines[rawLines.length - 1] === '') rawLines.pop();

  const lines: PropertiesLine[] = [];
  const values: Record<string, string> = {};
  for (let i = 0; i < rawLines.length; i++) {
    let raw = rawLines[i];
    const trimmed = raw.replace(/^[ \t\f]+/, '');
    if (trimmed === '' || trimmed.startsWith('#') || trimmed.startsWith('!')) {
      lines.push({ kind: 'other', raw });
      continue;
    }
    let logical = trimmed;
    while (continues(logical) && i + 1 < rawLines.length) {
      i++;
      raw += '\n' + rawLines[i];
      logical = logical.slice(0, -1) + rawLines[i].replace(/^[ \t\f]+/, '');
    }
    const { key, value } = splitLine(logical);
    lines.push({ kind: 'prop', raw, key, value });
    values[key] = value;
  }
  return { lines, values };
}

/**
 * Writes the file back: untouched lines stay byte for byte, changed keys are rewritten in place
 * and new keys go at the end.
 */
export function serializeProperties(parsed: ParsedProperties, changes: Record<string, string>): string {
  const pending = new Map(Object.entries(changes));
  const out: string[] = [];
  for (const line of parsed.lines) {
    if (line.kind === 'prop' && pending.has(line.key)) {
      out.push(`${line.key}=${escapeValue(pending.get(line.key) ?? '')}`);
      pending.delete(line.key);
    } else {
      out.push(line.raw);
    }
  }
  for (const [key, value] of pending) out.push(`${key}=${escapeValue(value)}`);
  return out.join('\n') + '\n';
}

// ---- MOTD formatting codes ----

export const MC_COLORS: Record<string, string> = {
  '0': '#000000', '1': '#0000AA', '2': '#00AA00', '3': '#00AAAA',
  '4': '#AA0000', '5': '#AA00AA', '6': '#FFAA00', '7': '#AAAAAA',
  '8': '#555555', '9': '#5555FF', a: '#55FF55', b: '#55FFFF',
  c: '#FF5555', d: '#FF55FF', e: '#FFFF55', f: '#FFFFFF',
};

export type MotdSegment = {
  text: string;
  color?: string;
  bold?: boolean;
  italic?: boolean;
  underline?: boolean;
  strike?: boolean;
  obfuscated?: boolean;
};

/** Splits a MOTD line into styled runs, following the § codes (including §x hex colours). */
export function parseMotdLine(line: string): MotdSegment[] {
  const segments: MotdSegment[] = [];
  let style: Omit<MotdSegment, 'text'> = {};
  let text = '';
  const flush = () => {
    if (text) segments.push({ ...style, text });
    text = '';
  };
  for (let i = 0; i < line.length; i++) {
    if (line[i] !== '§' || i + 1 >= line.length) {
      text += line[i];
      continue;
    }
    const code = line[i + 1].toLowerCase();
    // §x§r§r§g§g§b§b: a hex colour, one digit per §
    if (code === 'x' && /^(§[0-9a-fA-F]){6}$/.test(line.slice(i + 2, i + 14))) {
      flush();
      const hex = line.slice(i + 2, i + 14).replace(/§/g, '');
      style = { color: `#${hex}` };
      i += 13;
      continue;
    }
    if (MC_COLORS[code]) {
      flush();
      // a colour code also resets the formatting, as in the game
      style = { color: MC_COLORS[code] };
    } else if (code === 'l') { flush(); style = { ...style, bold: true }; }
    else if (code === 'o') { flush(); style = { ...style, italic: true }; }
    else if (code === 'n') { flush(); style = { ...style, underline: true }; }
    else if (code === 'm') { flush(); style = { ...style, strike: true }; }
    else if (code === 'k') { flush(); style = { ...style, obfuscated: true }; }
    else if (code === 'r') { flush(); style = {}; }
    else {
      text += line[i];
      continue;
    }
    i++;
  }
  flush();
  return segments;
}
