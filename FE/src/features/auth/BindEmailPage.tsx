import EmailVerificationForm from './EmailVerificationForm'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../store/auth'
import { returnPath } from './authForm'
export default function BindEmailPage() {
  const { user } = useAuth()
  const location = useLocation()
  if (user?.email_verified) return <Navigate to={returnPath(location.state?.from)} replace />
  return <EmailVerificationForm mode="bind" />
}
