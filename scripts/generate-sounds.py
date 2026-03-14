#!/usr/bin/env python3
"""
Generate notification WAV files for MapWatch severity levels.

No external dependencies — uses Python stdlib only (wave, struct, math).
Output goes to static/sounds/ relative to repo root.

Severity → sound design:
  critical  (Red)    — urgent double-beep  880 Hz, then 1108 Hz, 80 ms each
  high      (Orange) — single sharp beep   880 Hz, 120 ms
  medium    (Yellow) — gentle double-ping  660 → 784 Hz, 100 ms each
  low       (Green)  — soft chime          528 Hz, 200 ms (long sustain)
  info      (Blue)   — two-note rise       440 → 550 Hz, 80 ms each
  test      (Purple) — triple blip         330 → 440 → 550 Hz, 60 ms each
  debug     (White)  — quiet micro-click   220 Hz, 30 ms
"""

import math
import os
import struct
import wave

SAMPLE_RATE = 44100
OUT_DIR = os.path.join(os.path.dirname(__file__), '..', 'static', 'sounds')


def sine_wave(freq: float, duration_ms: int, amplitude: float = 0.4,
              fade_ms: int = 10) -> list[int]:
    """Return 16-bit PCM samples for a sine tone with linear fade-in/out."""
    n_samples = int(SAMPLE_RATE * duration_ms / 1000)
    fade_n = int(SAMPLE_RATE * fade_ms / 1000)
    samples = []
    for i in range(n_samples):
        t = i / SAMPLE_RATE
        val = math.sin(2 * math.pi * freq * t) * amplitude
        # Fade in
        if i < fade_n:
            val *= i / fade_n
        # Fade out
        if i > n_samples - fade_n:
            val *= (n_samples - i) / fade_n
        samples.append(int(val * 32767))
    return samples


def silence(duration_ms: int) -> list[int]:
    return [0] * int(SAMPLE_RATE * duration_ms / 1000)


def write_wav(filename: str, samples: list[int]) -> None:
    path = os.path.join(OUT_DIR, filename)
    with wave.open(path, 'w') as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(SAMPLE_RATE)
        wf.writeframes(struct.pack(f'<{len(samples)}h', *samples))
    print(f'  wrote {path}  ({len(samples)} samples)')


def main() -> None:
    os.makedirs(OUT_DIR, exist_ok=True)
    print(f'Generating sounds → {os.path.abspath(OUT_DIR)}')

    sounds = {
        # critical: urgent double-beep, higher pitch second hit
        'critical.wav': (
            sine_wave(880,  100, 0.92) + silence(35) +
            sine_wave(1108, 100, 0.92) + silence(20)
        ),
        # high: single sharp beep
        'high.wav': sine_wave(880, 150, 0.90),
        # medium: double-ping, rising
        'medium.wav': (
            sine_wave(660, 120, 0.88) + silence(30) +
            sine_wave(784, 120, 0.88)
        ),
        # low: chime with sustain
        'low.wav': sine_wave(528, 260, 0.85, fade_ms=40),
        # info: two-note rise
        'info.wav': (
            sine_wave(440, 100, 0.85) + silence(20) +
            sine_wave(550, 100, 0.85)
        ),
        # test: triple blip
        'test.wav': (
            sine_wave(330, 80, 0.88) + silence(20) +
            sine_wave(440, 80, 0.88) + silence(20) +
            sine_wave(550, 80, 0.88)
        ),
        # debug: audible click
        'debug.wav': sine_wave(330, 50, 0.75),
    }

    for filename, samples in sounds.items():
        write_wav(filename, samples)

    print('Done.')


if __name__ == '__main__':
    main()
