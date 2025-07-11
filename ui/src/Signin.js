import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useState } from 'react';
import Button from '@mui/material/Button';
import Card from '@mui/material/Card';
import Snackbar from '@mui/material/Snackbar';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';
import { GoogleAuthProvider, signInWithPopup } from 'firebase/auth';
import GoogleLogo from './assets/google_logo.svg';
import auth from './auth';
import { signInToVisionEx } from './grpcweb/client';
const SignIn = ({ setAuthenticated, }) => {
    const [openSnackbar, setOpenSnackbar] = useState(false);
    const closeSnackbar = (_, reason) => {
        if (reason === 'clickaway') {
            return;
        }
        setOpenSnackbar(false);
    };
    const signIn = async () => {
        try {
            const credential = await signInWithPopup(auth, new GoogleAuthProvider());
            const idToken = await credential.user.getIdToken();
            await signInToVisionEx(idToken);
            setAuthenticated(true);
        }
        catch (error) {
            console.error(error);
            setOpenSnackbar(true);
        }
    };
    return (_jsxs(Stack, { justifyContent: 'center', alignItems: 'center', sx: {
            height: '95vh',
            backgroundSize: 'cover',
            backgroundRepeat: 'no-repeat',
            backgroundPosition: 'center center',
        }, children: [_jsxs(Card, { component: Stack, spacing: 4, sx: { p: 5, width: 1, maxWidth: 420 }, children: [_jsx(Typography, { variant: 'h4', children: "Sign in to VisionEx" }), _jsxs(Button, { fullWidth: true, size: 'large', color: 'inherit', variant: 'outlined', sx: (theme) => ({
                            borderColor: alpha(theme.palette.grey[500], 0.16),
                            bgcolor: theme.palette.common.white,
                            display: 'flex',
                            columnGap: 2,
                        }), onClick: () => signIn(), children: [_jsx("img", { src: GoogleLogo, width: 20, height: 20, alt: 'Google logo' }), _jsx(Typography, { children: "Sign in with Google" })] })] }), _jsx(Snackbar, { open: openSnackbar, autoHideDuration: 6000, onClose: closeSnackbar, anchorOrigin: { vertical: 'top', horizontal: 'center' }, message: 'Failed to sign in. Please try again.' })] }));
};
export default SignIn;
