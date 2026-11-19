"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import {
  Camera,
  RotateCw,
  Pause,
  Play,
  Sparkles,
  ZoomIn,
  ZoomOut,
  X,
  Eye,
  Box,
} from "lucide-react";
import { optimizeTirePhoto } from "../lib/image-optimize";

export type TireViewerProps = {
  code: string;
  brand: string;
  model: string;
  size: string;
  treadMm: number;
  pressurePsi?: number | null;
  retreadCount?: number;
  status?: string;
  photoData?: string | null;
  canEdit?: boolean;
  savingPhoto?: boolean;
  onSavePhoto?: (photoData: string, photoThumb: string) => Promise<void>;
};

function createProceduralTreadTexture(
  photoImg: HTMLImageElement | null,
  code: string,
  treadMm: number,
): THREE.CanvasTexture {
  const canvas = document.createElement("canvas");
  canvas.width = 512;
  canvas.height = 512;
  const ctx = canvas.getContext("2d")!;

  if (photoImg) {
    ctx.drawImage(photoImg, 0, 0, 512, 512);
    ctx.fillStyle = "rgba(18, 26, 24, 0.22)";
    ctx.fillRect(0, 0, 512, 512);
  } else {
    ctx.fillStyle = "#1b2220";
    ctx.fillRect(0, 0, 512, 512);
  }

  // Realistic tread grooves & shoulder sipes
  ctx.strokeStyle = photoImg ? "rgba(12, 16, 15, 0.55)" : "#0d1211";
  ctx.lineWidth = 8;
  for (let y = 16; y < 512; y += 32) {
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(190, y + 14);
    ctx.lineTo(322, y - 14);
    ctx.lineTo(512, y);
    ctx.stroke();
  }
  // Longitudinal circumferential channels
  ctx.lineWidth = 10;
  [128, 256, 384].forEach((x) => {
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, 512);
    ctx.stroke();
  });

  // Subtle sidewall stencil watermark
  ctx.fillStyle = "rgba(210, 223, 213, 0.16)";
  ctx.font = "bold 24px Arial, sans-serif";
  ctx.fillText(`${code} • ${treadMm.toFixed(1)}mm`, 24, 480);

  const tex = new THREE.CanvasTexture(canvas);
  tex.wrapS = THREE.RepeatWrapping;
  tex.wrapT = THREE.RepeatWrapping;
  tex.repeat.set(3, 1);
  tex.colorSpace = THREE.SRGBColorSpace;
  return tex;
}

export function TireViewer360AR({
  code,
  brand,
  model,
  size,
  treadMm,
  pressurePsi,
  retreadCount = 0,
  status = "ok",
  photoData,
  canEdit = false,
  savingPhoto = false,
  onSavePhoto,
}: TireViewerProps) {
  const stageRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [autoSpin, setAutoSpin] = useState(true);
  const [zoom, setZoom] = useState(1);
  const [arMode, setArMode] = useState(false);
  const [arError, setArError] = useState("");
  const [viewMode, setViewMode] = useState<"3d" | "photo">("3d");
  const [photoBusy, setPhotoBusy] = useState(false);
  const [photoMessage, setPhotoMessage] = useState("");

  const rotRef = useRef({ x: 0.28, y: 0.45, vx: 0, vy: 0.008 });
  const autoSpinRef = useRef(autoSpin);
  const zoomRef = useRef(zoom);
  const draggingRef = useRef<{ active: boolean; x: number; y: number }>({
    active: false,
    x: 0,
    y: 0,
  });

  useEffect(() => {
    autoSpinRef.current = autoSpin;
  }, [autoSpin]);

  useEffect(() => {
    zoomRef.current = zoom;
  }, [zoom]);

  useEffect(() => {
    if (viewMode === "photo" && !arMode) return;
    const container = stageRef.current;
    if (!container) return;

    const width = container.clientWidth || 360;
    const height = container.clientHeight || 280;

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(38, width / height, 0.1, 100);
    camera.position.set(0, 0.35, 4.1);

    const renderer = new THREE.WebGLRenderer({
      antialias: true,
      alpha: true,
      powerPreference: "high-performance",
    });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.setSize(width, height);
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.18;
    container.innerHTML = "";
    container.appendChild(renderer.domElement);

    // Studio & rim lighting
    const ambient = new THREE.AmbientLight(0xffffff, 0.95);
    scene.add(ambient);

    const keyLight = new THREE.DirectionalLight(0xfff8ed, 2.1);
    keyLight.position.set(4, 6, 5);
    scene.add(keyLight);

    const rimLight = new THREE.DirectionalLight(0x7be0b4, 1.35);
    rimLight.position.set(-5, 2, -4);
    scene.add(rimLight);

    const fillLight = new THREE.DirectionalLight(0xd2dfd5, 0.75);
    fillLight.position.set(0, -4, 3);
    scene.add(fillLight);

    // Build realistic truck tire group
    const tireGroup = new THREE.Group();
    scene.add(tireGroup);

    const rubberMat = new THREE.MeshStandardMaterial({
      color: 0x222826,
      roughness: 0.78,
      metalness: 0.08,
    });

    // Main tire carcass (torus)
    const tireGeo = new THREE.TorusGeometry(0.95, 0.38, 32, 64);
    const tireMesh = new THREE.Mesh(tireGeo, rubberMat);
    tireGroup.add(tireMesh);

    // Outer tread band (cylinder) for crisp photo/tread mapping
    const treadGeo = new THREE.CylinderGeometry(1.32, 1.32, 0.56, 64, 1, true);
    treadGeo.rotateX(Math.PI / 2);
    const treadMat = new THREE.MeshStandardMaterial({
      color: 0xffffff,
      roughness: 0.82,
      metalness: 0.05,
    });
    const treadMesh = new THREE.Mesh(treadGeo, treadMat);
    tireGroup.add(treadMesh);

    // Steel/alloy rim
    const rimGeo = new THREE.CylinderGeometry(0.64, 0.64, 0.5, 48);
    rimGeo.rotateX(Math.PI / 2);
    const rimMat = new THREE.MeshStandardMaterial({
      color: 0xcfd8d3,
      roughness: 0.28,
      metalness: 0.82,
    });
    const rimMesh = new THREE.Mesh(rimGeo, rimMat);
    tireGroup.add(rimMesh);

    // Inner hub & lug nuts
    const hubGeo = new THREE.CylinderGeometry(0.26, 0.26, 0.58, 32);
    hubGeo.rotateX(Math.PI / 2);
    const hubMat = new THREE.MeshStandardMaterial({
      color: 0x293c38,
      roughness: 0.4,
      metalness: 0.65,
    });
    const hubMesh = new THREE.Mesh(hubGeo, hubMat);
    tireGroup.add(hubMesh);

    const nutGeo = new THREE.CylinderGeometry(0.045, 0.045, 0.56, 12);
    nutGeo.rotateX(Math.PI / 2);
    const nutMat = new THREE.MeshStandardMaterial({
      color: 0x8ea399,
      roughness: 0.25,
      metalness: 0.9,
    });
    for (let i = 0; i < 8; i++) {
      const angle = (i / 8) * Math.PI * 2;
      const nut = new THREE.Mesh(nutGeo, nutMat);
      nut.position.set(Math.cos(angle) * 0.44, Math.sin(angle) * 0.44, 0);
      tireGroup.add(nut);
    }

    // Load tire photo onto 3D tread & sidewall if available
    let disposed = false;
    let activeTex: THREE.Texture | null = null;
    const applyTexture = (img: HTMLImageElement | null) => {
      if (disposed) return;
      const tex = createProceduralTreadTexture(img, code, treadMm);
      activeTex?.dispose();
      activeTex = tex;
      treadMat.map = tex;
      treadMat.needsUpdate = true;
      if (img) {
        rubberMat.map = tex;
        rubberMat.needsUpdate = true;
      }
    };

    if (photoData) {
      const img = new Image();
      img.onload = () => applyTexture(img);
      img.onerror = () => applyTexture(null);
      img.src = photoData;
    } else {
      applyTexture(null);
    }

    const onResize = () => {
      if (!container) return;
      const w = container.clientWidth || 360;
      const h = container.clientHeight || 280;
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
      renderer.setSize(w, h);
    };
    window.addEventListener("resize", onResize);

    let raf = 0;
    const animate = () => {
      raf = requestAnimationFrame(animate);
      if (!draggingRef.current.active) {
        if (autoSpinRef.current) {
          rotRef.current.y += 0.008;
        } else {
          rotRef.current.y += rotRef.current.vy;
          rotRef.current.x += rotRef.current.vx;
          rotRef.current.vx *= 0.92;
          rotRef.current.vy *= 0.92;
        }
      }
      tireGroup.rotation.x = rotRef.current.x;
      tireGroup.rotation.y = rotRef.current.y;
      const s = zoomRef.current;
      tireGroup.scale.set(s, s, s);
      renderer.render(scene, camera);
    };
    animate();

    return () => {
      disposed = true;
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", onResize);
      activeTex?.dispose();
      tireGeo.dispose();
      treadGeo.dispose();
      rimGeo.dispose();
      hubGeo.dispose();
      nutGeo.dispose();
      rubberMat.dispose();
      treadMat.dispose();
      rimMat.dispose();
      hubMat.dispose();
      nutMat.dispose();
      renderer.dispose();
    };
  }, [photoData, code, treadMm, viewMode, arMode]);

  // Camera AR stream lifecycle
  useEffect(() => {
    if (!arMode) return;
    let stream: MediaStream | null = null;
    let active = true;
    setArError("");

    async function startCamera() {
      if (!navigator.mediaDevices?.getUserMedia) {
        setArError("Câmera AR indisponível neste navegador.");
        return;
      }
      try {
        stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: { ideal: "environment" } },
          audio: false,
        });
        if (!active) {
          stream.getTracks().forEach((t) => t.stop());
          return;
        }
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          await videoRef.current.play().catch(() => undefined);
        }
      } catch {
        if (active) {
          setArError(
            "Permita o acesso à câmera para projetar o pneu em Realidade Aumentada.",
          );
        }
      }
    }
    void startCamera();

    return () => {
      active = false;
      stream?.getTracks().forEach((t) => t.stop());
    };
  }, [arMode]);

  async function handleFileChange(file: File | undefined) {
    if (!file || !onSavePhoto) return;
    setPhotoBusy(true);
    setPhotoMessage("");
    try {
      const optimized = await optimizeTirePhoto(file);
      await onSavePhoto(optimized.photoData, optimized.photoThumb);
      setPhotoMessage(`Foto otimizada (${optimized.sizeKB} KB) salva com segurança!`);
    } catch (e) {
      setPhotoMessage((e as Error).message || "Falha ao otimizar foto.");
    } finally {
      setPhotoBusy(false);
    }
  }

  const startDrag = (clientX: number, clientY: number) => {
    draggingRef.current = { active: true, x: clientX, y: clientY };
  };
  const moveDrag = (clientX: number, clientY: number) => {
    if (!draggingRef.current.active) return;
    const dx = clientX - draggingRef.current.x;
    const dy = clientY - draggingRef.current.y;
    draggingRef.current.x = clientX;
    draggingRef.current.y = clientY;
    rotRef.current.y += dx * 0.012;
    rotRef.current.x = Math.max(-1.2, Math.min(1.2, rotRef.current.x + dy * 0.01));
    rotRef.current.vy = dx * 0.002;
    rotRef.current.vx = dy * 0.002;
  };
  const endDrag = () => {
    draggingRef.current.active = false;
  };

  return (
    <div className={arMode ? "tire-360-shell ar-active" : "tire-360-shell"}>
      <div className="tire-360-hud-top">
        <div className="tire-360-badges">
          <span className="hud-badge">
            <Sparkles size={13} />
            {arMode ? "REALIDADE AUMENTADA" : "VISÃO 360° INTERATIVA"}
          </span>
          <span className={`hud-status ${status}`}>
            {status === "critico" ? "Crítico" : status === "alerta" ? "Atenção" : "Saudável"}
          </span>
        </div>
        <div className="tire-360-mode-switch">
          {photoData && !arMode && (
            <button
              type="button"
              className={viewMode === "3d" ? "hud-btn active" : "hud-btn"}
              onClick={() => setViewMode(viewMode === "3d" ? "photo" : "3d")}
            >
              {viewMode === "3d" ? <Eye size={14} /> : <Box size={14} />}
              {viewMode === "3d" ? "Ver foto" : "Ver 3D"}
            </button>
          )}
          <button
            type="button"
            className={arMode ? "hud-btn ar-on" : "hud-btn"}
            onClick={() => {
              setArMode(!arMode);
              setViewMode("3d");
            }}
          >
            {arMode ? <X size={14} /> : <Camera size={14} />}
            {arMode ? "Sair do AR" : "Abrir em AR"}
          </button>
        </div>
      </div>

      <div
        className="tire-360-viewport"
        onPointerDown={(e) => startDrag(e.clientX, e.clientY)}
        onPointerMove={(e) => moveDrag(e.clientX, e.clientY)}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onPointerLeave={endDrag}
      >
        {arMode && (
          <video
            ref={videoRef}
            className="tire-ar-video"
            playsInline
            muted
            autoPlay
          />
        )}
        {viewMode === "photo" && photoData && !arMode ? (
          <div className="tire-photo-preview-wrap">
            <img src={photoData} alt={`Foto do pneu ${code}`} className="tire-photo-preview" />
          </div>
        ) : (
          <div ref={stageRef} className="tire-360-canvas" aria-label={`Visualizador 360 graus do pneu ${code}`} />
        )}

        {arError && <div className="tire-ar-error">{arError}</div>}
      </div>

      <div className="tire-360-telemetry-strip">
        <div>
          <small>PNEU</small>
          <strong>{code}</strong>
        </div>
        <div>
          <small>SULCO</small>
          <strong>{treadMm.toFixed(1)} mm</strong>
        </div>
        <div>
          <small>PRESSÃO</small>
          <strong>{pressurePsi ? `${pressurePsi} PSI` : "—"}</strong>
        </div>
        <div>
          <small>VIDA</small>
          <strong>{retreadCount + 1}ª vida</strong>
        </div>
      </div>

      <div className="tire-360-controls">
        <div className="tire-360-control-group">
          <button
            type="button"
            className="hud-icon-btn"
            aria-label={autoSpin ? "Pausar rotação 360" : "Girar 360"}
            onClick={() => setAutoSpin((v) => !v)}
          >
            {autoSpin ? <Pause size={15} /> : <Play size={15} />}
          </button>
          <button
            type="button"
            className="hud-icon-btn"
            aria-label="Aproximar"
            onClick={() => setZoom((z) => Math.min(1.65, +(z + 0.15).toFixed(2)))}
          >
            <ZoomIn size={15} />
          </button>
          <button
            type="button"
            className="hud-icon-btn"
            aria-label="Afastar"
            onClick={() => setZoom((z) => Math.max(0.65, +(z - 0.15).toFixed(2)))}
          >
            <ZoomOut size={15} />
          </button>
          <button
            type="button"
            className="hud-icon-btn"
            aria-label="Resetar ângulo"
            onClick={() => {
              rotRef.current = { x: 0.28, y: 0.45, vx: 0, vy: 0.008 };
              setZoom(1);
            }}
          >
            <RotateCw size={15} />
          </button>
        </div>

        <div className="tire-360-caption">
          <span>{brand} {model} · {size}</span>
          <small>Arraste para girar 360°</small>
        </div>

        {canEdit && onSavePhoto && (
          <div>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              capture="environment"
              hidden
              onChange={(e) => {
                const f = e.target.files?.[0];
                e.target.value = "";
                void handleFileChange(f);
              }}
            />
            <button
              type="button"
              className="button small"
              disabled={photoBusy || savingPhoto}
              onClick={() => fileInputRef.current?.click()}
            >
              <Camera size={15} />
              {photoBusy || savingPhoto
                ? "Otimizando…"
                : photoData
                  ? "Atualizar foto"
                  : "Tirar foto do pneu"}
            </button>
          </div>
        )}
      </div>
      {photoMessage && <p className="tire-photo-feedback" role="status">{photoMessage}</p>}
    </div>
  );
}
