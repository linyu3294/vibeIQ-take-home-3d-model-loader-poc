import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom'
import { useState } from 'react'
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

function UploadDialog({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  if (!isOpen) return null;

  return (
    <div className="dialog-overlay">
      <div className="dialog">
        <button className="dialog-close" onClick={onClose}>×</button>
        <h2>Upload 3D Model</h2>
        <div className="dialog-content">
          <input type="file" accept=".glb,.gltf" />
          <button className="upload-button">Upload</button>
        </div>
      </div>
    </div>
  );
}

function Gallery() {
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  return (
    <div className="gallery">
      <div className="gallery-header">
        <button className="upload-button" onClick={() => setIsDialogOpen(true)}>Upload</button>
      </div>
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
      <UploadDialog isOpen={isDialogOpen} onClose={() => setIsDialogOpen(false)} />
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
