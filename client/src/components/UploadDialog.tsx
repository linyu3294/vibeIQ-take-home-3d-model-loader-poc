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

export default UploadDialog; 