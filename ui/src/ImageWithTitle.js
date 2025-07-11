import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Box, Typography } from '@mui/material';
const ImageWithTitle = ({ src, alt, title }) => {
    return (_jsxs(Box, { textAlign: 'center', m: 2, children: [_jsx("img", { src: src, alt: alt, style: {
                    maxWidth: '200px',
                    maxHeight: '200px',
                    display: 'block',
                    margin: 'auto',
                } }), _jsx(Typography, { variant: 'body1', sx: { mt: 1 }, children: title })] }));
};
export default ImageWithTitle;
