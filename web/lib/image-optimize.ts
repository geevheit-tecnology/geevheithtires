export type OptimizedTirePhoto = {
  photoData: string;
  photoThumb: string;
  sizeKB: number;
};

function encodeCanvas(
  canvas: HTMLCanvasElement,
  quality: number,
): string {
  const webp = canvas.toDataURL("image/webp", quality);
  if (webp.startsWith("data:image/webp;base64,")) {
    return webp;
  }
  return canvas.toDataURL("image/jpeg", quality);
}

function drawScaled(
  img: HTMLImageElement,
  maxDimension: number,
): HTMLCanvasElement {
  const scale = Math.min(1, maxDimension / Math.max(img.width, img.height));
  const width = Math.max(1, Math.round(img.width * scale));
  const height = Math.max(1, Math.round(img.height * scale));
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Canvas 2D indisponível neste dispositivo.");
  ctx.drawImage(img, 0, 0, width, height);
  return canvas;
}

export async function optimizeTirePhoto(file: File): Promise<OptimizedTirePhoto> {
  if (!file.type.startsWith("image/")) {
    throw new Error("Selecione um arquivo de imagem válido.");
  }
  const objectUrl = URL.createObjectURL(file);
  try {
    const img = await new Promise<HTMLImageElement>((resolve, reject) => {
      const el = new Image();
      el.onload = () => resolve(el);
      el.onerror = () => reject(new Error("Não foi possível ler a imagem selecionada."));
      el.src = objectUrl;
    });

    let maxDim = 760;
    let mainCanvas = drawScaled(img, maxDim);
    let quality = 0.76;
    let photoData = encodeCanvas(mainCanvas, quality);
    while (photoData.length > 180_000 && quality > 0.35) {
      quality -= 0.1;
      photoData = encodeCanvas(mainCanvas, quality);
    }
    while (photoData.length > 190_000 && maxDim > 360) {
      maxDim = Math.round(maxDim * 0.78);
      mainCanvas = drawScaled(img, maxDim);
      photoData = encodeCanvas(mainCanvas, 0.62);
    }

    const thumbCanvas = drawScaled(img, 96);
    let photoThumb = encodeCanvas(thumbCanvas, 0.6);
    if (photoThumb.length > 18_500) {
      photoThumb = encodeCanvas(drawScaled(img, 72), 0.5);
    }

    const approxBytes = Math.round((photoData.length * 3) / 4);
    return {
      photoData,
      photoThumb,
      sizeKB: Math.max(1, Math.round(approxBytes / 1024)),
    };
  } finally {
    URL.revokeObjectURL(objectUrl);
  }
}
