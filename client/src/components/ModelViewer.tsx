import { useParams } from 'react-router-dom';
import { useRef, useEffect, useState } from 'react';
import * as THREE from 'three';
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
import type { GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js';

function ModelViewer() {
  const { id } = useParams<{ id: string }>();
  const containerRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<THREE.Scene | null>(null);
  const cameraRef = useRef<THREE.PerspectiveCamera | null>(null);
  const rendererRef = useRef<THREE.WebGLRenderer | null>(null);
  const controlsRef = useRef<OrbitControls | null>(null);
  const apiKey = import.meta.env.VITE_API_KEY;
  const apiUrl = import.meta.env.VITE_API_HTTPS_URL;
  const [loadingProgress, setLoadingProgress] = useState(0);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    if (!containerRef.current || !id) return;

    // Set initial loading state
    setIsLoading(true);
    setLoadingProgress(0);

    // Set container to fill viewport
    containerRef.current.style.position = 'fixed';
    containerRef.current.style.top = '0';
    containerRef.current.style.left = '0';
    containerRef.current.style.width = '100vw';
    containerRef.current.style.height = '100vh';
    containerRef.current.style.margin = '0';
    containerRef.current.style.padding = '0';
    containerRef.current.style.background = '#808080';

    // Initialize Three.js scene
    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0x808080);
    sceneRef.current = scene;

    const camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
    camera.position.set(0, 0, 5);
    cameraRef.current = camera;

    const renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setSize(window.innerWidth, window.innerHeight);
    renderer.setPixelRatio(window.devicePixelRatio);
    containerRef.current.appendChild(renderer.domElement);
    rendererRef.current = renderer;

    // Add OrbitControls
    const controls = new OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    controls.dampingFactor = 0.05;
    controlsRef.current = controls;

    // Add lights
    const ambientLight = new THREE.AmbientLight(0xffffff, 0.7);
    scene.add(ambientLight);
    const directionalLight = new THREE.DirectionalLight(0xffffff, 1);
    directionalLight.position.set(5, 10, 7.5);
    scene.add(directionalLight);


    // Load model
    const loadModel = async () => {
      try {
        const response = await fetch(`${apiUrl}/3d-model/${id}?getPresignedUploadURL=false&fileType=glb`, {
          headers: {
            'x-api-key': apiKey,
            'Content-Type': 'application/json'
          }
        });
        const data = await response.json();
        console.log('Received data:', data);

        const loader = new GLTFLoader();
        console.log('Starting model load...');
        loader.load(
          data.presignedUrl,
          (gltf: GLTF) => {
            console.log('Model loaded successfully');
            // Center the model
            const box = new THREE.Box3().setFromObject(gltf.scene);
            const center = box.getCenter(new THREE.Vector3());
            gltf.scene.position.sub(center);

            // Scale up small models
            const size = box.getSize(new THREE.Vector3());
            const maxDim = Math.max(size.x, size.y, size.z);
            let scale = 1;
            if (maxDim < 2) {
              scale = 2 / maxDim; // Make smallest models at least 2 units big
              gltf.scene.scale.setScalar(scale);
            }

            scene.add(gltf.scene);

            // Add padding (10% of maxDim or window size)
            const padding = 0.15; // 15% padding
            const paddedDim = maxDim * (1 + padding);
            const fov = camera.fov * (Math.PI / 180);
            // Calculate camera distance so model fits with padding
            const cameraZ = paddedDim / (2 * Math.tan(fov / 2));
            camera.position.set(0, 0, cameraZ);
            camera.near = cameraZ / 100;
            camera.far = cameraZ * 100;
            camera.updateProjectionMatrix();
            camera.lookAt(0, 0, 0);

            // Update controls target
            controls.target.set(0, 0, 0);
            controls.update();
            
            // Reset loading states when done
            setLoadingProgress(0);
            setIsLoading(false);
          },
          (progress: ProgressEvent) => {
            const progressPercent = (progress.loaded / progress.total * 100);
            console.log('Loading progress:', progressPercent + '%');
            setLoadingProgress(progressPercent);
          },
          (error: unknown) => {
            console.error('Error loading model:', error);
            setLoadingProgress(0);
            setIsLoading(false);
          }
        );
      } catch (error) {
        console.error('Error fetching model URL:', error);
        setIsLoading(false);
      }
    };

    loadModel();

    // Animation loop
    const animate = () => {
      requestAnimationFrame(animate);
      controls.update();
      renderer.render(scene, camera);
    };
    
    animate();

    // Handle window resize
    const handleResize = () => {
      if (!camera || !renderer) return;
      camera.aspect = window.innerWidth / window.innerHeight;
      camera.updateProjectionMatrix();
      renderer.setSize(window.innerWidth, window.innerHeight);
    };
    window.addEventListener('resize', handleResize);

    // Cleanup
    return () => {
      window.removeEventListener('resize', handleResize);
      if (containerRef.current && renderer.domElement) {
        containerRef.current.removeChild(renderer.domElement);
      }
      renderer.dispose();
    };
  }, [id]);

  return (
    <>
      <div ref={containerRef} style={{ 
        width: '100vw', 
        height: '100vh', 
        margin: 0, 
        padding: 0, 
        background: '#808080',
        position: 'relative',
        zIndex: 1
      }} />
      {(isLoading || loadingProgress > 0) && (
        <div style={{
          position: 'fixed',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          background: 'rgba(0, 0, 0, 0.8)',
          padding: '20px',
          borderRadius: '10px',
          color: 'white',
          fontFamily: 'Arial, sans-serif',
          zIndex: 9999,
          pointerEvents: 'none',
          minWidth: '200px',
          textAlign: 'center'
        }}>
          <div style={{ marginBottom: '10px' }}>
            {loadingProgress > 0 
              ? `Loading Model: ${Math.round(loadingProgress)}%`
              : 'Preparing to load model...'}
          </div>
          <div style={{
            width: '100%',
            height: '10px',
            background: '#444',
            borderRadius: '5px',
            overflow: 'hidden'
          }}>
            <div style={{
              width: `${loadingProgress}%`,
              height: '100%',
              background: '#4CAF50',
              transition: 'width 0.3s ease-in-out'
            }} />
          </div>
        </div>
      )}
    </>
  );
}

export default ModelViewer; 