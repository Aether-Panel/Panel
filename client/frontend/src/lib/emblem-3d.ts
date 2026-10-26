import * as THREE from 'three';
import { SVGLoader } from 'three/examples/jsm/loaders/SVGLoader.js';
import { mergeGeometries } from 'three/examples/jsm/utils/BufferGeometryUtils.js';
import { RoomEnvironment } from 'three/examples/jsm/environments/RoomEnvironment.js';

/*
 * The Aether emblem in real relief for the sign-in brand column, built from
 * /img/emblem.svg the same way services.aetherpanel.es does it: a black plate
 * (the outlines read as engraved grooves) with every coloured piece extruded on
 * top in its own material, gold, enamel blue and porcelain.
 *
 * Building it is heavy (the SVG has thousands of curves), so the work is cut in
 * slices that hand control back to the browser and the page stays responsive.
 */

const SIZE = 3.3; // emblem width in scene units
const PLATE = 0.1;
const PIECE = 0.11;
const LAYER_STEP = 0.0012;
// curves in the drawing are a few units long out of 2976, so 4 divisions look identical
const CURVE_DIVISIONS = 4;
const CORNER_DIVISIONS = 4;
const MIN_DISTANCE = 1;

type Layer = 'black' | 'gold' | 'blue' | 'ivory';

function layerOf(fill: string | undefined): Layer | null {
    switch ((fill ?? '').toUpperCase()) {
        case '#000':
        case '#000000':
            return 'black';
        case '#DDBD63':
            return 'gold';
        case '#379AC3':
            return 'blue';
        case '#F1F0EB':
            return 'ivory';
        default:
            return null;
    }
}

type WithScheduler = { scheduler?: { yield?: () => Promise<void> } };

function yieldToBrowser(): Promise<void> {
    const scheduler = (globalThis as WithScheduler).scheduler;
    if (typeof scheduler?.yield === 'function') {
        return scheduler.yield();
    }
    return new Promise((resolve) => setTimeout(resolve, 0));
}

/** Yields only once the current slice has run for more than `ms`. */
function yieldEvery(ms = 12) {
    let since = performance.now();
    return async () => {
        if (performance.now() - since < ms) {
            return;
        }
        await yieldToBrowser();
        since = performance.now();
    };
}

// Flipping the y axis reverses the triangles' winding; swap it back so light hits the front
function flipFaces(geometry: THREE.BufferGeometry) {
    if (geometry.index) {
        const index = geometry.index.array;
        for (let i = 0; i < index.length; i += 3) {
            const tmp = index[i + 1];
            index[i + 1] = index[i + 2];
            index[i + 2] = tmp;
        }
        geometry.index.needsUpdate = true;
        return;
    }
    for (const attribute of Object.values(geometry.attributes) as THREE.BufferAttribute[]) {
        const n = attribute.itemSize;
        const values = attribute.array;
        for (let v = 0; v + 2 < attribute.count; v += 3) {
            for (let k = 0; k < n; k++) {
                const a = (v + 1) * n + k;
                const b = (v + 2) * n + k;
                const tmp = values[a];
                values[a] = values[b];
                values[b] = tmp;
            }
        }
        attribute.needsUpdate = true;
    }
}

async function buildEmblem(isCancelled: () => boolean): Promise<{ group: THREE.Group; dispose: () => void } | null> {
    const svg = await fetch('/img/emblem.svg').then((r) => r.text());
    const next = yieldEvery();
    await next();
    const data = new SVGLoader().parse(svg);
    const viewBox = /viewBox="0 0 (\d+) (\d+)"/.exec(svg);
    const side = viewBox ? Number(viewBox[1]) : 2976;
    const scale = SIZE / side;

    const pieces: Record<Layer, THREE.BufferGeometry[]> = { black: [], gold: [], blue: [], ivory: [] };
    const strokes: THREE.BufferGeometry[] = [];

    let order = 0;
    for (const path of data.paths) {
        await next();
        if (isCancelled()) {
            return null;
        }
        const style = path.userData?.style as { fill?: string; stroke?: string } | undefined;
        const layer = layerOf(style?.fill);
        if (!layer) {
            continue;
        }
        const depth = layer === 'black' ? PLATE : PIECE;
        const z = layer === 'black' ? 0 : PLATE + order++ * LAYER_STEP;
        for (const shape of path.toShapes()) {
            await next();
            const geometry = new THREE.ExtrudeGeometry(shape, { depth: depth / scale, bevelEnabled: false, curveSegments: 3 });
            geometry.translate(0, 0, z / scale);
            pieces[layer].push(geometry);
        }
        // the black outline's 44-unit stroke widens the silhouette: draw it on the plate's face
        if (layer === 'black' && style?.stroke && style.stroke !== 'none') {
            for (const sub of path.subPaths) {
                await next();
                const geometry = SVGLoader.pointsToStroke(
                    sub.getPoints(CURVE_DIVISIONS),
                    path.userData!.style as Parameters<typeof SVGLoader.pointsToStroke>[1],
                    CORNER_DIVISIONS,
                    MIN_DISTANCE,
                );
                if (geometry) {
                    geometry.translate(0, 0, PLATE / scale + 0.5);
                    strokes.push(geometry);
                }
            }
        }
    }

    const materials: Record<Layer, THREE.Material> = {
        black: new THREE.MeshBasicMaterial({ color: 0x07090c }),
        gold: new THREE.MeshPhysicalMaterial({ color: 0xe3aa2b, metalness: 1, roughness: 0.26, clearcoat: 0.5, clearcoatRoughness: 0.2 }),
        blue: new THREE.MeshPhysicalMaterial({ color: 0x1670ab, roughness: 0.35, metalness: 0, clearcoat: 0.8, clearcoatRoughness: 0.1, envMapIntensity: 0.3 }),
        ivory: new THREE.MeshPhysicalMaterial({ color: 0xeae7dc, roughness: 0.45, metalness: 0, clearcoat: 0.5, clearcoatRoughness: 0.15, envMapIntensity: 0.45 }),
    };

    const group = new THREE.Group();
    const geometries: THREE.BufferGeometry[] = [];
    const place = (geometry: THREE.BufferGeometry) => {
        // SVG coordinates (y down, origin top left) to the centre of the scene
        geometry.translate(-side / 2, -side / 2, 0);
        geometry.scale(scale, -scale, scale);
        flipFaces(geometry);
        geometries.push(geometry);
        return geometry;
    };

    for (const layer of Object.keys(pieces) as Layer[]) {
        if (pieces[layer].length === 0) {
            continue;
        }
        await next();
        const merged = mergeGeometries(pieces[layer], false);
        pieces[layer].forEach((g) => g.dispose());
        if (merged) {
            group.add(new THREE.Mesh(place(merged), materials[layer]));
        }
    }
    if (strokes.length > 0) {
        await next();
        const merged = mergeGeometries(strokes, false);
        strokes.forEach((g) => g.dispose());
        if (merged) {
            group.add(new THREE.Mesh(place(merged), materials.black));
        }
    }

    return {
        group,
        dispose: () => {
            geometries.forEach((g) => g.dispose());
            Object.values(materials).forEach((m) => m.dispose());
        },
    };
}

type MountOptions = {
    canvas: HTMLCanvasElement;
    /** Called once the emblem is on screen, so the flat fallback can fade out. */
    onReady: () => void;
    /** No usable WebGL: the flat emblem stays. */
    onFail: () => void;
};

/**
 * Mounts the floating emblem on the canvas and returns the function that tears it down
 * (safe to call while it is still being built). It drifts gently and leans towards the
 * pointer on a spring; with reduced motion it renders one still frame.
 */
export function mountEmblem({ canvas, onReady, onFail }: MountOptions): () => void {
    let cancelled = false;
    const cleanups: (() => void)[] = [];
    const unmount = () => {
        cancelled = true;
        cleanups.reverse().forEach((fn) => fn());
        cleanups.length = 0;
    };

    let renderer: THREE.WebGLRenderer;
    try {
        renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true, powerPreference: 'high-performance' });
    } catch {
        onFail();
        return unmount;
    }
    cleanups.push(() => renderer.dispose());
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.05;

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(30, 1, 0.1, 40);
    camera.position.set(0, 0, 7.4);
    const key = new THREE.DirectionalLight(0xfff3dc, 2.4);
    key.position.set(-3, 5, 6);
    scene.add(key);
    const rim = new THREE.DirectionalLight(0x9cc8ff, 1.1);
    rim.position.set(4, -2, -4);
    scene.add(rim);
    scene.add(new THREE.AmbientLight(0xffffff, 0.35));
    const holder = new THREE.Group();
    scene.add(holder);

    const resize = () => {
        const w = canvas.clientWidth;
        const h = canvas.clientHeight;
        if (w === 0 || h === 0) {
            return;
        }
        renderer.setSize(w, h, false);
        camera.aspect = w / h;
        camera.updateProjectionMatrix();
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(canvas);
    cleanups.push(() => observer.disconnect());

    const still = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    // pointer on a spring, desktop only
    const target = { x: 0, y: 0 };
    const pos = { x: 0, y: 0 };
    const vel = { x: 0, y: 0 };
    const onMove = (e: PointerEvent) => {
        target.x = (e.clientX / window.innerWidth) * 2 - 1;
        target.y = (e.clientY / window.innerHeight) * 2 - 1;
    };
    if (!still && window.matchMedia('(hover: hover) and (pointer: fine)').matches) {
        window.addEventListener('pointermove', onMove, { passive: true });
        cleanups.push(() => window.removeEventListener('pointermove', onMove));
    }

    const prepare = async () => {
        await yieldToBrowser();
        if (cancelled) return;
        // a lit room, prefiltered once, gives the gold something to reflect
        const pmrem = new THREE.PMREMGenerator(renderer);
        const environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture;
        scene.environment = environment;
        pmrem.dispose();
        cleanups.push(() => environment.dispose());

        const built = await buildEmblem(() => cancelled);
        if (!built || cancelled) {
            built?.dispose();
            return;
        }
        cleanups.push(built.dispose);
        holder.add(built.group);
        await renderer.compileAsync(scene, camera);
        if (cancelled) return;

        const start = performance.now();
        let last = start;
        let frame = 0;
        const draw = (t: number) => {
            const dt = Math.min((t - last) / 1000, 1 / 30);
            last = t;
            const s = (t - start) / 1000;
            for (const k of ['x', 'y'] as const) {
                vel[k] += ((target[k] - pos[k]) * 60 - vel[k] * 12) * dt;
                pos[k] += vel[k] * dt;
            }
            // arrives with a short rise, then floats and sways a little
            const enter = still ? 1 : 1 - Math.pow(1 - Math.min(1, s / 1.1), 4);
            holder.position.y = (still ? 0 : Math.sin(s * 0.7) * 0.06) - (1 - enter) * 0.35;
            holder.rotation.set(pos.y * 0.12, (still ? -0.18 : Math.sin(s * 0.35) * 0.22) + pos.x * 0.3, 0);
            holder.scale.setScalar(0.001 + enter * 0.999);
            renderer.render(scene, camera);
        };

        if (still) {
            draw(start);
        } else {
            const loop = (t: number) => {
                frame = requestAnimationFrame(loop);
                if (document.hidden) {
                    last = t;
                    return;
                }
                draw(t);
            };
            frame = requestAnimationFrame(loop);
            cleanups.push(() => cancelAnimationFrame(frame));
        }
        onReady();
    };

    prepare().catch(() => {
        if (!cancelled) onFail();
    });

    return unmount;
}
