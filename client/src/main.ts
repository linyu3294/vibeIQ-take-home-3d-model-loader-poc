import * as THREE from 'three';
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
import type { GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js';

// Environment variables
const API_GATEWAY_URL = import.meta.env.VITE_API_GATEWAY_URL;
const API_KEY = import.meta.env.VITE_API_KEY;

// Three.js setup
const scene = new THREE.Scene();
scene.background = new THREE.Color(0x808080); // Light gray background

const camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
camera.position.set(5, 5, 5); // Position camera at an angle

const renderer = new THREE.WebGLRenderer({ antialias: true });
renderer.setSize(window.innerWidth, window.innerHeight);
renderer.setPixelRatio(window.devicePixelRatio);
document.body.appendChild(renderer.domElement);

// Add OrbitControls
const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = true; // Add smooth damping effect
controls.dampingFactor = 0.05;

// Add lights
const ambientLight = new THREE.AmbientLight(0xffffff, 0.5);
scene.add(ambientLight);
const directionalLight = new THREE.DirectionalLight(0xffffff, 1);
directionalLight.position.set(5, 5, 5);
scene.add(directionalLight);

// Load GLB model
async function loadModel() {
    try {
        // Fetch model URL from API
        const response = await fetch(`${API_GATEWAY_URL}/3d-model/1E1D33E1-76F1-455A-8E37-A82AC5D2568F`, {
            headers: {
                'x-api-key': `${API_KEY}`,
                'Content-Type': 'application/json'
            }
        });
        const data = await response.json();
        console.log('Received data:', data);

        // Load the GLB file using the pre-signed URL
        const loader = new GLTFLoader();
        loader.load(
            data.modelUrl, // Use modelUrl from the response
            (gltf: GLTF) => {
                scene.add(gltf.scene);
                
                // Center the model
                const box = new THREE.Box3().setFromObject(gltf.scene);
                const center = box.getCenter(new THREE.Vector3());
                gltf.scene.position.sub(center);
                
                // Adjust camera to fit model
                const size = box.getSize(new THREE.Vector3());
                const maxDim = Math.max(size.x, size.y, size.z);
                const fov = camera.fov * (Math.PI / 180);
                let cameraZ = Math.abs(maxDim / Math.sin(fov / 2));
                camera.position.set(cameraZ, cameraZ, cameraZ);
                camera.lookAt(center);
                
                // Update controls target
                controls.target.copy(center);
                controls.update();
            },
            (progress: ProgressEvent) => {
                console.log('Loading progress:', (progress.loaded / progress.total * 100) + '%');
            },
            (error: unknown) => {
                console.error('Error loading model:', error);
            }
        );
    } catch (error) {
        console.error('Error fetching model URL:', error);
    }
}

// Animation loop
function animate() {
    requestAnimationFrame(animate);
    controls.update(); // Update controls in animation loop
    renderer.render(scene, camera);
}
animate();

// Load the model when the page loads
loadModel();

// Handle window resize
window.addEventListener('resize', () => {
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
    renderer.setSize(window.innerWidth, window.innerHeight);
});

