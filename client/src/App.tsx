import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom'
import { useState, useRef } from 'react'
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

  const handleUploadClick = () => {
    setIsDialogOpen(true);
    setSelectedFile(null);
  };

  const handleFileUpload = async (file: File) => {
    setUploading(true);
    setWaitingForWS(false);
    connectionIdSet.current = false;
    setConnectionId(null);

    // Open WebSocket connection for this upload session
    const ws = new WebSocket('wss://tok3wpajoh.execute-api.us-east-1.amazonaws.com/prod?x-api-key=1e84e4522ebec480c6280684355d05bc9137b2ad40553dfae3ab156c1c4ca531');
    wsRef.current = ws;
    const modelId = file.name.replace(/\.blend$/, '');
    const apiKey = '1e84e4522ebec480c6280684355d05bc9137b2ad40553dfae3ab156c1c4ca531';

    // Promise to wait for connectionId
    let connectionIdPromiseResolve: (value: string) => void;
    const connectionIdPromise = new Promise<string>(resolve => {
      connectionIdPromiseResolve = resolve;
    });

    ws.onopen = () => {
      console.log('WebSocket connected');
      ws.send(JSON.stringify({ action: 'init' }));
      // 6) 1st get call to get .blend presign url
      fetch(`https://2imojbde0f.execute-api.us-east-1.amazonaws.com/v1/3d-model/${modelId}?getPresignedUploadURL=true&fileType=blend`, {
        method: 'GET',
        headers: { 'x-api-key': apiKey },
      })
        .then(getResp => {
          if (!getResp.ok) throw new Error('Failed to get presigned URL');
          return getResp.json();
        })
        .then(({ presignedUrl }) => {
          if (!presignedUrl) throw new Error('No presignedUrl in response');
          // 7) put call to update the presign url with .blend file
          return fetch(presignedUrl, {
            method: 'PUT',
            body: file,
            headers: {},
          });
        })
        .then(putResp => {
          if (!putResp.ok) throw new Error('Failed to upload file to presigned URL');
          // 8) Wait for connectionId before POST
          return connectionIdPromise;
        })
        .then((connId) => {
          const s3Key = `blend/${modelId}.blend`;
          return fetch(`https://2imojbde0f.execute-api.us-east-1.amazonaws.com/v1/3d-model`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'x-api-key': apiKey,
            },
            body: JSON.stringify({
              connectionId: connId,
              fromFileType: 'blend',
              toFileType: 'glb',
              modelId,
              s3Key,
            }),
          });
        })
        .then(postResp => {
          if (!postResp.ok) throw new Error('Failed to POST to API');
          // 9) Websocket notifies client (handled in ws.onmessage)
          setWaitingForWS(true);
        })
        .catch(err => {
          setUploading(false);
          setWaitingForWS(false);
          console.error('Upload failed:', err);
          if (wsRef.current) {
            wsRef.current.close();
            wsRef.current = null;
          }
          connectionIdSet.current = false;
          setConnectionId(null);
        });
    };
    ws.onmessage = (event) => {
      console.log('WebSocket message received:', event.data);
      try {
        const data = JSON.parse(event.data);
        if (data.connectionId && !connectionIdSet.current) {
          setConnectionId(data.connectionId);
          connectionIdSet.current = true;
          connectionIdPromiseResolve(data.connectionId); // <-- resolve the promise
          console.log('Received connectionId:', data.connectionId);
        } else if (data.jobStatus === 'completed') {
          setUploading(false);
          setWaitingForWS(false);
          // 10) Client makes the second get call to get the .glb
          if (wsRef.current) {
            wsRef.current.close();
            wsRef.current = null;
          }
          connectionIdSet.current = false;
          setConnectionId(null);
          const modelId = data.modelId || (selectedFile ? selectedFile.name.replace(/\.blend$/, '') : '');
          const getUrl = `https://2imojbde0f.execute-api.us-east-1.amazonaws.com/v1/3d-model/${modelId}?getPresignedUploadURL=false&fileType=glb`;
          fetch(getUrl, {
            method: 'GET',
            headers: { 'x-api-key': apiKey },
          })
            .then(resp => resp.json())
            .then(data => {
              console.log('GET after job completion:', data);
            })
            .catch(err => {
              console.error('Error in GET after job completion:', err);
            });
        }
      } catch (e) {
        console.log('Non-JSON WebSocket message:', event.data, e);
        if (wsRef.current) {
          wsRef.current.close();
          wsRef.current = null;
        }
        connectionIdSet.current = false;
        setConnectionId(null);
      }
    };
    ws.onerror = (err) => {
      console.error('WebSocket error:', err);
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      connectionIdSet.current = false;
      setConnectionId(null);
      setUploading(false);
      setWaitingForWS(false);
    };
    ws.onclose = () => {
      console.log('WebSocket closed');
      connectionIdSet.current = false;
      setConnectionId(null);
    };
    setIsDialogOpen(false);
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
