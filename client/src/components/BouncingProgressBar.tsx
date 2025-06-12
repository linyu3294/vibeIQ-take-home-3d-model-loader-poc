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

export default BouncingProgressBar; 