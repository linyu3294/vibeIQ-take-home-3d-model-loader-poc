import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom'
import { useState, useRef, useEffect } from 'react'
import './App.css'

// Mock data
const models = [
  { id: '1E1D33E1-76F1-455A-8E37-A82AC5D2568F', fileType: 'glb', imageUrl: 'https://images.unsplash.com/photo-1748854091034-abd9d3ea6be8?q=80&w=1374&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D' },
  { id: '2F2E44F2-87G2-566B-9F48-B93BD6E3679G', fileType: 'glb', imageUrl: 'https://images.unsplash.com/photo-1748024093647-bbbfbe2c0c3f?q=80&w=1394&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D' },
  { id: '3G3F55G3-98H3-677C-0G59-C04CE7F4780H', fileType: 'glb', imageUrl: 'https://images.unsplash.com/photo-1748682170760-aba4b59da534?q=80&w=1374&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D' },
]

function ModelCard({ id, fileType, imageUrl }: { id: string; fileType: string; imageUrl: string }) {
  return (
    <Link to={`/model/${id}`} className="card">
      <img src={imageUrl} alt={`Model ${id}`} className="card-image" />
      <div className="card-content">
        <p>Type: {fileType}</p>
        <p>ID: {id}</p>
      </div>
    </Link>
  )
}

function BouncingProgressBar({ visible }: { visible: boolean }) {
  if (!visible) return null;
  return (
    <div style={{ width: '100%', height: 6, background: '#eee', margin: '10px 0', position: 'relative', overflow: 'hidden' }}>
      <div className="bouncing-bar" style={{
        width: 80,
        height: '100%',
        background: '#4caf50',
        position: 'absolute',
        animation: 'bounce 1.2s infinite cubic-bezier(.4,0,.6,1)'
      }} />
      <style>{`
        @keyframes bounce {
          0% { left: 0; }
          50% { left: calc(100% - 80px); }
          100% { left: 0; }
        }
      `}</style>
    </div>
  );
}

function UploadDialog({ isOpen, onClose, onUpload, uploading, selectedFile, setSelectedFile }: {
  isOpen: boolean;
  onClose: () => void;
  onUpload: (file: File) => void;
  uploading: boolean;
  selectedFile: File | null;
  setSelectedFile: (file: File | null) => void;
}) {
  if (!isOpen) return null;
  return (
    <div className="dialog-overlay">
      <div className="dialog">
        <button className="dialog-close" onClick={onClose}>×</button>
        <h2>Upload 3D Model</h2>
        <div className="dialog-content">
          <input type="file" accept=".blend" onChange={e => setSelectedFile(e.target.files?.[0] || null)} />
          <button className="upload-button" onClick={() => selectedFile && onUpload(selectedFile)} disabled={!selectedFile || uploading}>
            {uploading ? 'Uploading...' : 'Upload'}
          </button>
        </div>
      </div>
    </div>
  );
}

function Gallery() {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [connectionId, setConnectionId] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [waitingForWS, setWaitingForWS] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const connectionIdSet = useRef(false);

  useEffect(() => {
    // Open WebSocket connection once on mount
    const ws = new WebSocket('wss://tok3wpajoh.execute-api.us-east-1.amazonaws.com/prod?x-api-key=1e84e4522ebec480c6280684355d05bc9137b2ad40553dfae3ab156c1c4ca531');
    wsRef.current = ws;
    ws.onopen = () => {
      console.log('WebSocket connected');
      ws.send(JSON.stringify({ action: 'init' }));
    };
    ws.onmessage = (event) => {
      console.log('WebSocket message received:', event.data); // Catch-all log
      try {
        const data = JSON.parse(event.data);
        if (data.connectionId && !connectionIdSet.current) {
          setConnectionId(data.connectionId);
          connectionIdSet.current = true;
          console.log('Received connectionId:', data.connectionId);
        } else if (data.jobStatus === 'completed') {
          setUploading(false);
          setWaitingForWS(false);
          // Optionally, show a success message or update UI
        }
      } catch (e) {
        console.log('Non-JSON WebSocket message:', event.data, e);
      }
    };
    ws.onerror = (err) => {
      console.error('WebSocket error:', err);
    };
    ws.onclose = () => {
      console.log('WebSocket closed');
      connectionIdSet.current = false;
      setConnectionId(null);
    };
    return () => {
      ws.close();
    };
  }, []);

  const handleUploadClick = () => {
    setIsDialogOpen(true);
    setSelectedFile(null);
  };

  const handleFileUpload = async (file: File) => {
    if (!connectionId || !wsRef.current || wsRef.current.readyState !== 1) {
      console.error('WebSocket not connected or connectionId not available');
      return;
    }
    if (!file.name.endsWith('.blend')) {
      console.error('Only .blend files are allowed');
      return;
    }
    setIsDialogOpen(false);
    setUploading(true);
    setWaitingForWS(false);
    const modelId = file.name.replace(/\.blend$/, '');
    const apiKey = '1e84e4522ebec480c6280684355d05bc9137b2ad40553dfae3ab156c1c4ca531';
    try {
      // First call: GET presigned URL
      const getUrl = `https://2imojbde0f.execute-api.us-east-1.amazonaws.com/v1/3d-model/${modelId}?getPresignedUploadURL=true&fileType=blend`;
      const getResp = await fetch(getUrl, {
        method: 'GET',
        headers: { 'x-api-key': apiKey },
      });
      if (!getResp.ok) throw new Error('Failed to get presigned URL');
      const { presignedUrl } = await getResp.json();
      if (!presignedUrl) throw new Error('No presignedUrl in response');
      // Second call: PUT file to presigned URL
      const putResp = await fetch(presignedUrl, {
        method: 'PUT',
        body: file,
        headers: {},
      });
      if (!putResp.ok) throw new Error('Failed to upload file to presigned URL');
      // Third call: POST to API
      const postUrl = `https://2imojbde0f.execute-api.us-east-1.amazonaws.com/v1/3d-model`;
      const s3Key = `blend/${modelId}.blend`;
      const postBody = {
        connectionId,
        fromFileType: 'blend',
        toFileType: 'glb',
        modelId,
        s3Key,
      };
      const postResp = await fetch(postUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'x-api-key': apiKey,
        },
        body: JSON.stringify(postBody),
      });
      if (!postResp.ok) throw new Error('Failed to POST to API');
      // Wait for WebSocket message
      setWaitingForWS(true);
    } catch (err) {
      setUploading(false);
      setWaitingForWS(false);
      console.error('Upload failed:', err);
    }
  };

  return (
    <div className="gallery">
      <div className="gallery-header">
        <button className="upload-button" onClick={handleUploadClick}>Upload</button>
        {connectionId && (
          <div style={{ marginTop: 10, color: 'green' }}>Connection ID: {connectionId}</div>
        )}
      </div>
      <BouncingProgressBar visible={uploading || waitingForWS} />
      <UploadDialog isOpen={isDialogOpen} onClose={() => setIsDialogOpen(false)} onUpload={handleFileUpload} uploading={uploading || waitingForWS} selectedFile={selectedFile} setSelectedFile={setSelectedFile} />
      <div className="cards-grid">
        {models.map((model) => (
          <ModelCard key={model.id} {...model} />
        ))}
      </div>
      <div className="pagination">
        <a href="#" className="pagination-link">&lt;&lt; Previous | </a>
        <a href="#" className="pagination-link">Page 1</a>
        <a href="#" className="pagination-link"> | Next &gt;&gt;</a>
      </div>
    </div>
  )
}

function ModelViewer() {
  return (
    <div className="model-viewer">
      <h1>Model Viewer</h1>
      {/* Three.js viewer will go here */}
    </div>
  )
}

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<Gallery />} />
        <Route path="/model/:id" element={<ModelViewer />} />
      </Routes>
    </Router>
  )
}

export default App
