import React, { useMemo, useEffect, useRef } from 'react';
import { OrbitControls, PerspectiveCamera, Points, PointMaterial } from '@react-three/drei';
import * as THREE from 'three';

const palette = {
  fatal: new THREE.Color('#8b0000'),
  warn: new THREE.Color('#00ff00'),
  info: new THREE.Color('#0000ff'),
  default: new THREE.Color('#ffd700')
};

// Stable jitter to keep selection rings centered
const getStableJitter = (id, amount) => {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = id.charCodeAt(i) + ((hash << 5) - hash);
  }
  return (Math.abs(hash % 100) / 100 - 0.5) * amount;
};

function getLogColor(p) {
  const level = (p.level || "").toUpperCase();
  if (level === 'FATAL' || level === 'ERROR') return palette.fatal;
  if (level === 'WARN') return palette.warn;
  if (level === 'INFO' || level === 'DEBUG') return palette.info;
  return palette.default;
}

export function Galaxy({ points, selectedId }) {
  const controlsRef = useRef();
  
  const { positions, colors, targetCoords } = useMemo(() => {
    if (!points?.length) return { positions: new Float32Array([0,0,0]), colors: new Float32Array([1,1,1]), targetCoords: null };

    const coords = new Float32Array(points.length * 3);
    const colArray = new Float32Array(points.length * 3);
    let foundTarget = null;

    points.forEach((p, i) => {
      // Use stable jitter based on ID so dots don't move
      const sX = getStableJitter(p.id || String(i), 2.0);
      const sY = getStableJitter((p.id || String(i)) + 'y', 2.0);
      const sZ = getStableJitter((p.id || String(i)) + 'z', 2.0);

      const x = (p.x * 20) + sX;
      const y = (p.y * 20) + sY;
      const z = (p.z * 20) + sZ;

      coords[i * 3] = x;
      coords[i * 3 + 1] = y;
      coords[i * 3 + 2] = z;

      const dotColor = getLogColor(p);
      const isSelected = selectedId && p.id && p.id.replace(/-/g, '') === selectedId.replace(/-/g, '');

      if (isSelected) {
        colArray[i * 3] = 1; colArray[i * 3 + 1] = 1; colArray[i * 3 + 2] = 1;
        foundTarget = new THREE.Vector3(x, y, z);
      } else {
        colArray[i * 3] = dotColor.r; colArray[i * 3 + 1] = dotColor.g; colArray[i * 3 + 2] = dotColor.b;
      }
    });

    return { positions: coords, colors: colArray, targetCoords: foundTarget };
  }, [points, selectedId]);

  // AUTO-FOCUS LOGIC: Moves the camera's rotation center to the selected ball
  useEffect(() => {
    if (targetCoords && controlsRef.current) {
      const controls = controlsRef.current;
      // Smoothly move the rotation center to the outlier
      controls.target.lerp(targetCoords, 1.0); 
      controls.update();
    } else if (!selectedId && controlsRef.current) {
      // Return to center when nothing is selected
      controlsRef.current.target.set(0, 0, 0);
      controlsRef.current.update();
    }
  }, [targetCoords, selectedId]);

  return (
    <>
      <PerspectiveCamera makeDefault position={[0, 10, 30]} fov={50} near={0.01} />
      
      <OrbitControls 
        ref={controlsRef}
        enablePan={true} 
        makeDefault 
        minDistance={0.5} 
        maxDistance={100} 
      />

      <gridHelper args={[40, 80, '#06b6d4', '#022c22']} position={[0, -1, 0]} />

      <Points positions={positions} colors={colors} stride={3} frustumCulled={false}>
        <PointMaterial
          transparent
          vertexColors
          size={0.18}
          sizeAttenuation
          depthWrite={false}
          blending={THREE.AdditiveBlending}
          alphaTest={0.5}
        />
      </Points>

      {targetCoords && (
        <group position={targetCoords}>
          {/* Glowing Target Ring */}
          <mesh rotation={[Math.PI / 2, 0, 0]}>
            <ringGeometry args={[0.4, 0.5, 32]} />
            <meshBasicMaterial color="#ffffff" transparent opacity={0.8} side={THREE.DoubleSide} />
          </mesh>
          {/* Wireframe Box */}
          <mesh>
            <boxGeometry args={[0.8, 0.8, 0.8]} />
            <meshBasicMaterial color="#ffffff" wireframe transparent opacity={0.3} />
          </mesh>
        </group>
      )}
    </>
  );
}