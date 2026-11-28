export type SoundKind = "message" | "call";
export type SoundMode = "default" | "preset" | "custom" | "mute";

export interface SoundPresetOption {
  id: string;
  label: string;
}

export const MESSAGE_SOUND_PRESETS: SoundPresetOption[] = [
  { id: "chime", label: "Chime Lembut" },
  { id: "crystal", label: "Crystal" },
  { id: "pop", label: "Pop Cepat" },
  { id: "bell", label: "Lonceng Jernih" },
];

export const CALL_SOUND_PRESETS: SoundPresetOption[] = [
  { id: "classic", label: "Classic Ring" },
  { id: "marimba", label: "Marimba" },
  { id: "pulse", label: "Pulse" },
  { id: "gentle", label: "Gentle Tone" },
];

interface NoteSpec {
  start: number;
  dur: number;
  freq: number;
  freqEnd?: number;
  harmonic?: number;
  gain: number;
}

const SAMPLE_RATE = 22050;

const PRESET_NOTES: Record<string, NoteSpec[]> = {
  "message:chime": [
    { start: 0, dur: 0.14, freq: 659.25, gain: 0.28 },
    { start: 0.11, dur: 0.26, freq: 987.77, gain: 0.26 },
  ],
  "message:crystal": [
    { start: 0, dur: 0.12, freq: 880, gain: 0.24 },
    { start: 0.08, dur: 0.14, freq: 1174.66, gain: 0.24 },
    { start: 0.17, dur: 0.28, freq: 1567.98, gain: 0.22 },
  ],
  "message:pop": [{ start: 0, dur: 0.09, freq: 523.25, freqEnd: 880, gain: 0.3 }],
  "message:bell": [{ start: 0, dur: 0.42, freq: 1046.5, harmonic: 2, gain: 0.26 }],
  "call:classic": [
    { start: 0.0, dur: 0.14, freq: 783.99, gain: 0.28 },
    { start: 0.16, dur: 0.14, freq: 987.77, gain: 0.28 },
    { start: 0.32, dur: 0.14, freq: 783.99, gain: 0.28 },
    { start: 0.48, dur: 0.28, freq: 987.77, gain: 0.28 },
  ],
  "call:marimba": [
    { start: 0.0, dur: 0.16, freq: 523.25, gain: 0.3 },
    { start: 0.14, dur: 0.16, freq: 659.25, gain: 0.28 },
    { start: 0.28, dur: 0.16, freq: 783.99, gain: 0.28 },
    { start: 0.44, dur: 0.3, freq: 1046.5, gain: 0.26 },
  ],
  "call:pulse": [
    { start: 0.0, dur: 0.18, freq: 698.46, gain: 0.28 },
    { start: 0.26, dur: 0.18, freq: 698.46, gain: 0.28 },
    { start: 0.52, dur: 0.22, freq: 880.0, gain: 0.28 },
  ],
  "call:gentle": [
    { start: 0.0, dur: 0.28, freq: 440, gain: 0.22 },
    { start: 0.24, dur: 0.3, freq: 554.37, gain: 0.22 },
    { start: 0.5, dur: 0.42, freq: 659.25, gain: 0.2 },
  ],
};

export function createPresetWavUrl(kind: SoundKind, preset: string): string {
  const key = `${kind}:${preset}`;
  const notes = PRESET_NOTES[key] ?? PRESET_NOTES[kind === "call" ? "call:classic" : "message:chime"] ?? [];
  let totalSec = 0.2;
  for (const n of notes) totalSec = Math.max(totalSec, n.start + n.dur);
  const totalSamples = Math.max(1, Math.ceil(totalSec * SAMPLE_RATE));
  const buffer = new ArrayBuffer(44 + totalSamples * 2);
  const view = new DataView(buffer);
  const writeStr = (offset: number, text: string) => {
    for (let i = 0; i < text.length; i++) view.setUint8(offset + i, text.charCodeAt(i));
  };
  writeStr(0, "RIFF");
  view.setUint32(4, 36 + totalSamples * 2, true);
  writeStr(8, "WAVE");
  writeStr(12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, SAMPLE_RATE, true);
  view.setUint32(28, SAMPLE_RATE * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeStr(36, "data");
  view.setUint32(40, totalSamples * 2, true);

  for (let i = 0; i < totalSamples; i++) {
    const t = i / SAMPLE_RATE;
    let sample = 0;
    for (const note of notes) {
      if (!note.freq || t < note.start || t > note.start + note.dur) continue;
      const local = t - note.start;
      const progress = local / note.dur;
      const env = Math.min(1, local / 0.012) * Math.exp(-3.4 * progress);
      const freq = note.freqEnd ? note.freq + (note.freqEnd - note.freq) * progress : note.freq;
      sample += Math.sin(2 * Math.PI * freq * local) * note.gain * env;
      if (note.harmonic) {
        sample += Math.sin(2 * Math.PI * freq * note.harmonic * local) * (note.gain * 0.35) * env;
      }
    }
    const clamped = Math.max(-1, Math.min(1, sample));
    view.setInt16(44 + i * 2, Math.round(clamped * 32767), true);
  }
  return URL.createObjectURL(new Blob([buffer], { type: "audio/wav" }));
}
