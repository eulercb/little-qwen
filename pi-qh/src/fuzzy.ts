// Port of qh's fuzzyReplace (internal/tools/tools.go, EXP-1 "fuzzy_edit"):
// when old_text is not found verbatim, match it line by line ignoring leading
// and trailing whitespace (must be unique), then re-indent new_text into the
// file's indentation style. Here it rewrites a pi edit's oldText/newText before
// the built-in edit tool runs, so pi applies an exact replacement.

export interface FuzzyFix {
  oldText: string; // exact text from the file
  newText: string; // re-indented replacement
  line: number; // 1-based line of the match
  note: string;
}

const leadingWS = (x: string) => x.slice(0, x.length - x.replace(/^[ \t]+/, "").length);

// Indentation width with tabs counted as 4 columns.
function wsCols(ws: string): number {
  let n = 0;
  for (const c of ws) n += c === "\t" ? 4 : 1;
  return n;
}

const trimNL = (s: string) => s.replace(/^\n+|\n+$/g, "");

export function fuzzyFix(file: string, oldT: string, newT: string): FuzzyFix | undefined {
  const fl = file.split("\n");
  const ol = trimNL(oldT).split("\n");
  if (ol.length === 0 || ol.join("").trim() === "") return undefined;
  let match = -1;
  for (let i = 0; i + ol.length <= fl.length; i++) {
    let ok = true;
    for (let j = 0; j < ol.length; j++) {
      if (fl[i + j].trim() !== ol[j].trim()) {
        ok = false;
        break;
      }
    }
    if (ok) {
      if (match >= 0) return undefined; // ambiguous
      match = i;
    }
  }
  if (match < 0) return undefined;

  // Re-base indentation: measure new_text's indentation in levels relative to
  // old_text's first non-blank line, then re-emit those levels in the file's style.
  let oldInd = "";
  let fileInd = "";
  for (let j = 0; j < ol.length; j++) {
    if (ol[j].trim() !== "") {
      oldInd = leadingWS(ol[j]);
      fileInd = leadingWS(fl[match + j]);
      break;
    }
  }
  const oldBase = wsCols(oldInd);
  let oldUnit = 0;
  for (const l of ol) {
    if (l.trim() === "") continue;
    const d = wsCols(leadingWS(l)) - oldBase;
    if (d > 0 && (oldUnit === 0 || d < oldUnit)) oldUnit = d;
  }
  if (oldUnit === 0) oldUnit = 4;
  const fileUnit = fileInd.includes("\t") || file.includes("\n\t") ? "\t" : "    ";
  const nl = trimNL(newT) === "" ? [] : trimNL(newT).split("\n");
  for (let k = 0; k < nl.length; k++) {
    const line = nl[k];
    if (line.trim() === "") continue;
    const d = wsCols(leadingWS(line)) - oldBase;
    const body = line.replace(/^[ \t]+/, "");
    if (d >= 0) {
      nl[k] = fileInd + fileUnit.repeat(Math.floor((d + Math.floor(oldUnit / 2)) / oldUnit)) + body;
    } else {
      let ind = fileInd;
      for (let n = Math.floor((-d + Math.floor(oldUnit / 2)) / oldUnit); n > 0 && ind !== ""; n--) {
        ind = ind.endsWith(fileUnit) ? ind.slice(0, ind.length - fileUnit.length) : ind.slice(0, -1);
      }
      nl[k] = ind + body;
    }
  }
  const note =
    oldInd !== fileInd
      ? `matched ignoring whitespace; re-indented from ${JSON.stringify(oldInd)} to ${JSON.stringify(fileInd)}`
      : "matched ignoring whitespace";
  return { oldText: fl.slice(match, match + ol.length).join("\n"), newText: nl.join("\n"), line: match + 1, note };
}

/** Number of non-overlapping occurrences (strings.Count semantics). */
export function countOf(s: string, sub: string): number {
  if (sub === "") return 0;
  let n = 0;
  for (let i = s.indexOf(sub); i >= 0; i = s.indexOf(sub, i + sub.length)) n++;
  return n;
}
