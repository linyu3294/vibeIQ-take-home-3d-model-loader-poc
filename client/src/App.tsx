import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom'
import { useState, useRef, useEffect } from 'react'
import './App.css'

type ModelMetadata = {
  jobId: string;
  connectionId: string;
  jobType: string;
  jobStatus: string;
  fromFileType: string;
  toFileType: string;
  modelId: string;
  s3Key: string;
  newS3Key?: string;
  error?: string;
  timestamp: string;
};

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
  const [models, setModels] = useState<ModelMetadata[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  const [page, setPage] = useState(1);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [connectionId, setConnectionId] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [waitingForWS, setWaitingForWS] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const connectionIdSet = useRef(false);
  const limit = 12;
  const apiKey = '1e84e4522ebec480c6280684355d05bc9137b2ad40553dfae3ab156c1c4ca531';

  const fetchModels = async (cursorParam: string | null) => {
    setLoading(true);
    setError(null);
    try {
      let url = `https://2imojbde0f.execute-api.us-east-1.amazonaws.com/v1/3d-models?fileType=glb&limit=${limit}`;
      if (cursorParam) {
        const encodedCursor = encodeURIComponent(cursorParam);
        url += `&cursor=${encodedCursor}`;
      }
      console.log('Request URL:', url);
      const resp = await fetch(url, {
        headers: { 'x-api-key': apiKey }
      });
      if (!resp.ok) {
        const errorData = await resp.json();
        throw new Error(errorData.error || 'Failed to fetch models');
      }
      const data = await resp.json();
      setModels(data.models || []);
      setNextCursor(data.nextCursor || null);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Unknown error');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchModels(cursor);
    // eslint-disable-next-line
  }, [cursor]);

  const handleNext = () => {
    if (nextCursor) {
      setPrevCursors(prev => [...prev, cursor || '']);
      setCursor(nextCursor);
      setPage(p => p + 1);
    }
  };

  const handlePrev = () => {
    if (prevCursors.length > 0) {
      const prev = [...prevCursors];
      const prevCursor = prev.pop() || null;
      setPrevCursors(prev);
      setCursor(prevCursor);
      setPage(p => p - 1);
    }
  };

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
      <BouncingProgressBar visible={loading || uploading || waitingForWS} />
      <UploadDialog isOpen={isDialogOpen} onClose={() => setIsDialogOpen(false)} onUpload={handleFileUpload} uploading={uploading || waitingForWS} selectedFile={selectedFile} setSelectedFile={setSelectedFile} />
      {error && <div style={{ color: 'red' }}>{error}</div>}
      <div className="cards-grid">
        {models.map((model) => (
          <ModelCard
            key={model.jobId}
            id={model.modelId}
            fileType={model.toFileType}
            imageUrl={'/placeholder-image.png'} // Replace with your actual image logic if available
          />
        ))}
      </div>
      <div className="pagination">
        <button className="pagination-link" onClick={handlePrev} disabled={page === 1 || loading}>&lt;&lt; Previous</button>
        <span className="pagination-link">Page {page}</span>
        <button className="pagination-link" onClick={handleNext} disabled={!nextCursor || loading}>Next &gt;&gt;</button>
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
