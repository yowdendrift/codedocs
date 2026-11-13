// UnrealBloomPass tuned to glow like three.js r128, the version the mockup was made with.
// Since then the pass multiplies its result by 3 and blurs with wider kernels, so the
// mockup's settings (strength 0.9, radius 0.55, threshold 0.1) wash everything out to white.
// This puts back r128's blur (sigma = kernel radius, normalised) and scales the strength down.

import { ShaderMaterial, Vector2 } from 'three';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';

const KERNELS = [3, 5, 7, 9, 11];
// Matched by eye against the mockup in r128, in the 3D and Top views. The newer pass composites
// with 3x gain in half-float targets where r128 clipped each stage at 1.
const STRENGTH = 1 / 7;

function blur(kernel: number, size: Vector2): ShaderMaterial {
  return new ShaderMaterial({
    defines: { KERNEL_RADIUS: kernel, SIGMA: kernel },
    uniforms: { colorTexture: { value: null }, invSize: { value: size }, direction: { value: new Vector2(0.5, 0.5) } },
    vertexShader: 'varying vec2 vUv; void main() { vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
    fragmentShader: `
      varying vec2 vUv;
      uniform sampler2D colorTexture;
      uniform vec2 invSize;
      uniform vec2 direction;
      float gaussianPdf(in float x, in float sigma) { return 0.39894 * exp(-0.5 * x * x / (sigma * sigma)) / sigma; }
      void main() {
        float fSigma = float(SIGMA);
        float weightSum = gaussianPdf(0.0, fSigma);
        vec3 diffuseSum = texture2D(colorTexture, vUv).rgb * weightSum;
        for (int i = 1; i < KERNEL_RADIUS; i++) {
          float x = float(i);
          float w = gaussianPdf(x, fSigma);
          vec2 uvOffset = direction * invSize * x;
          diffuseSum += (texture2D(colorTexture, vUv + uvOffset).rgb + texture2D(colorTexture, vUv - uvOffset).rgb) * w;
          weightSum += 2.0 * w;
        }
        gl_FragColor = vec4(diffuseSum / weightSum, 1.0);
      }`,
  });
}

export class MockupBloomPass extends UnrealBloomPass {
  constructor(resolution: Vector2, strength: number, radius: number, threshold: number) {
    super(resolution, strength * STRENGTH, radius, threshold);
    const mats = this.separableBlurMaterials as ShaderMaterial[];
    mats.forEach((m, i) => {
      const size = m.uniforms.invSize.value as Vector2; // setSize() replaces it on resize
      m.dispose();
      mats[i] = blur(KERNELS[i], size);
    });
  }
}
