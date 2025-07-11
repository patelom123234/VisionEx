import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import ImageWithTitle from './ImageWithTitle';
import { translateToImage } from './grpcweb/client';
import { Language } from './type';
const ImageToImage = ({ state, updateState, }) => {
    const readFile = (file) => {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => {
                if (reader.result instanceof ArrayBuffer) {
                    const imageBuffer = new Uint8Array(reader.result);
                    const url = URL.createObjectURL(new Blob([imageBuffer], { type: 'image/png' }));
                    resolve({
                        name: file.name,
                        imageBuffer,
                        url,
                    });
                }
                else {
                    reject(new Error('Failed to read file.'));
                }
            };
            reader.onerror = () => reject(new Error('File reading failed.'));
            reader.readAsArrayBuffer(file);
        });
    };
    const handleImageSelection = async (event) => {
        const files = event.target.files;
        if (!files || files.length === 0)
            return;
        updateState({ isLoading: true });
        try {
            const newImage = await readFile(files[0]);
            updateState({
                image: newImage,
                isLoading: false,
            });
        }
        catch (error) {
            console.error('Error reading file:', error);
            updateState({ isLoading: false });
        }
    };
    const toImage = async () => {
        updateState({ isLoading: true });
        try {
            if (!state.image) {
                throw new Error('No image selected');
            }
            const imageData = await translateToImage(state.image.imageBuffer, state.selectedLanguage);
            updateState({
                translatedImage: imageData,
                isLoading: false,
            });
        }
        catch (error) {
            console.error('Error translating image:', error);
            if (error instanceof Error && error.message === 'ResourceExhausted') {
                alert('Too many requests. Please try again later.');
            }
            updateState({ isLoading: false });
        }
    };
    return (_jsxs(Stack, { alignItems: 'center', sx: {
            py: 10,
            rowGap: 4,
        }, children: [_jsx(Box, { textAlign: 'center', display: 'flex', flexDirection: 'row', gap: 2, style: { marginBottom: '20px' }, children: _jsxs(Button, { variant: 'contained', component: 'label', disabled: state.isLoading, children: ["Select Image", _jsx("input", { type: 'file', accept: 'image/png, image/jpeg, image/jpg', hidden: true, onChange: handleImageSelection })] }) }), _jsx(Box, { display: 'flex', flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'center', gap: 2, children: state.image && (_jsx(ImageWithTitle, { src: state.image.url || '', alt: state.image.name, title: state.image.name })) }), state.image && (_jsx(Box, { display: 'flex', flexDirection: 'row', gap: 4, alignItems: 'center', children: _jsxs(Box, { display: 'flex', flexDirection: 'row', gap: 2, justifyContent: 'center', children: [_jsx(Typography, { variant: 'body1', sx: { display: 'flex', alignItems: 'center' }, children: "Result Language:" }), _jsxs(Select, { value: state.selectedLanguage, onChange: (e) => updateState({
                                selectedLanguage: e.target.value,
                            }), children: [_jsx(MenuItem, { value: Language.LANGUAGE_EN_US, children: "English" }), _jsx(MenuItem, { value: Language.LANGUAGE_KO_KR, children: "Korean" }), _jsx(MenuItem, { value: Language.LANGUAGE_JA_JP, children: "Japanese" })] }), _jsx(Button, { onClick: toImage, variant: 'contained', disabled: state.isLoading, children: "To Image" })] }) })), state.isLoading ? (_jsxs(Box, { textAlign: 'center', sx: { mt: 4 }, children: [_jsx(CircularProgress, {}), _jsx(Typography, { variant: 'body1', sx: { mt: 1 }, children: "Loading image..." })] })) : (state.translatedImage && (_jsx("img", { src: state.translatedImage, alt: 'Translated', style: { objectFit: 'contain' } })))] }));
};
export default ImageToImage;
