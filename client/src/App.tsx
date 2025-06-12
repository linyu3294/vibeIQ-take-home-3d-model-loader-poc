import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import './App.css'
import Gallery from './components/Gallery'
import ModelViewer from './components/ModelViewer'

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
