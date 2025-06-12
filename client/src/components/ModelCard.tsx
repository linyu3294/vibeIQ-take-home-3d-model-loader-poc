import { Link } from 'react-router-dom';

function ModelCard({ id, fileType }: { id: string; fileType: string; imageUrl: string }) {
  return (
    <Link to={`/model/${id}`} className="card" style={{ alignItems: 'center', justifyContent: 'center' }}>
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 140, width: '100%' }}>
        <img
          src="/3d-mesh-icon.png"
          alt={`Model ${id}`}
          style={{ maxWidth: 80, maxHeight: 80, width: 'auto', height: 'auto', display: 'block', margin: '0 auto' }}
        />
      </div>
      <div className="card-content" style={{ textAlign: 'center', marginTop: 16 }}>
        <p>Type: {fileType}</p>
        <p>ID: {id}</p>
      </div>
    </Link>
  );
}

export default ModelCard; 