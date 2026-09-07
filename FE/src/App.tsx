import { Routes, Route } from 'react-router-dom'
import RequireAuth from './components/RequireAuth'
import SessionGate from './components/SessionGate'
import HomePage from './features/home/HomePage'
import LoginPage from './features/auth/LoginPage'
import RegisterPage from './features/auth/RegisterPage'
import BindEmailPage from './features/auth/BindEmailPage'
import PlanningPage from './features/planning/PlanningPage'
import TripPage from './features/trip/TripPage'
import MyTripsPage from './features/trips/MyTripsPage'
import PlacesPage from './features/places/PlacesPage'
import AdminPlacesPage from './features/admin/AdminPlacesPage'

export default function App() {
  return (
    <SessionGate>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/bind-email" element={<RequireAuth><BindEmailPage /></RequireAuth>} />
        <Route path="/planning/:jobId" element={<RequireAuth><PlanningPage /></RequireAuth>} />
        <Route path="/trips" element={<RequireAuth><MyTripsPage /></RequireAuth>} />
        <Route path="/trips/:id" element={<RequireAuth><TripPage /></RequireAuth>} />
        <Route path="/places" element={<RequireAuth><PlacesPage /></RequireAuth>} />
        <Route path="/admin/places" element={<RequireAuth><AdminPlacesPage /></RequireAuth>} />
      </Routes>
    </SessionGate>
  )
}
