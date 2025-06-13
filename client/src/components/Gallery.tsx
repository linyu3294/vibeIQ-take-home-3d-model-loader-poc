import { useState, useRef, useEffect } from 'react';
import ModelCard from './ModelCard';
import BouncingProgressBar from './BouncingProgressBar';
import UploadDialog from './UploadDialog';

export type ModelMetadata = {
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
  const apiUrl = import.meta.env.VITE_API_HTTPS_URL;
  const webSocketUrl = import.meta.env.VITE_API_WEBSOCKET_URL;
  const apiKey = import.meta.env.VITE_API_KEY;

  const fetchModels = async (cursorParam: string | null) => {
    setLoading(true);
    setError(null);
    try {
      let url = `${apiUrl}/3d-models?fileType=glb&limit=${limit}`;
      if (cursorParam) {
        url += `&cursor=${encodeURIComponent(cursorParam)}`;
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
    const ws = new WebSocket(`${webSocketUrl}?x-api-key=${apiKey}`);
    wsRef.current = ws;
    const modelId = file.name.replace(/\.blend$/, '');

    // Promise to wait for connectionId
    let connectionIdPromiseResolve: (value: string) => void;
    const connectionIdPromise = new Promise<string>(resolve => {
      connectionIdPromiseResolve = resolve;
    });

    ws.onopen = () => {
      console.log('WebSocket connected');
      ws.send(JSON.stringify({ action: 'init' }));
      // 6) 1st get call to get .blend presign url
      fetch(`${apiUrl}/3d-model/${modelId}?getPresignedUploadURL=true&fileType=blend`, {
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
          return fetch(`${apiUrl}/3d-model`, {
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
          const getUrl = `${apiUrl}/3d-model/${modelId}?getPresignedUploadURL=false&fileType=glb`;
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
            imageUrl={'/3d-mesh-icon.png'}
          />
        ))}
      </div>
      <div className="pagination">
        <button className="pagination-link" onClick={handlePrev} disabled={page === 1 || loading}>&lt;&lt; Previous</button>
        <span className="pagination-link">Page {page}</span>
        <button className="pagination-link" onClick={handleNext} disabled={!nextCursor || loading}>Next &gt;&gt;</button>
      </div>
    </div>
  );
}

export default Gallery; 