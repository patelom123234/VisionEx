import React from 'react';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import ImageWithTitle from './ImageWithTitle';
import { translateToImage } from './grpcweb/client';
import { Language, TabState } from './type';

const ImageToImage = ({
  state,
  updateState,
}: {
  state: TabState['image'];
  updateState: (newState: Partial<TabState['image']>) => void;
}) => {
  const readFile = (
    file: File,
  ): Promise<{ name: string; imageBuffer: Uint8Array; url: string }> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();

      reader.onload = () => {
        if (reader.result instanceof ArrayBuffer) {
          const imageBuffer = new Uint8Array(reader.result);
          const url = URL.createObjectURL(
            new Blob([imageBuffer], { type: 'image/png' }),
          );
          resolve({
            name: file.name,
            imageBuffer,
            url,
          });
        } else {
          reject(new Error('Failed to read file.'));
        }
      };

      reader.onerror = () => reject(new Error('File reading failed.'));
      reader.readAsArrayBuffer(file);
    });
  };

  const handleImageSelection = async (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const files = event.target.files;
    if (!files || files.length === 0) return;

    updateState({ isLoading: true });
    try {
      const newImage = await readFile(files[0]);
      updateState({
        image: newImage,
        isLoading: false,
      });
    } catch (error) {
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
      const imageData = await translateToImage(
        state.image.imageBuffer,
        state.selectedLanguage,
      );
      updateState({
        translatedImage: imageData,
        isLoading: false,
      });
    } catch (error) {
      console.error('Error translating image:', error);
      if (error instanceof Error && error.message === 'ResourceExhausted') {
        alert('Too many requests. Please try again later.');
      }
      updateState({ isLoading: false });
    }
  };

  return (
    <Stack
      alignItems='center'
      sx={{
        py: 10,
        rowGap: 4,
      }}
    >
      <Box
        textAlign='center'
        display='flex'
        flexDirection='row'
        gap={2}
        style={{ marginBottom: '20px' }}
      >
        <Button
          variant='contained'
          component='label'
          disabled={state.isLoading}
        >
          Select Image
          <input
            type='file'
            accept='image/png, image/jpeg, image/jpg'
            hidden
            onChange={handleImageSelection}
          />
        </Button>
      </Box>
      <Box
        display='flex'
        flexDirection='row'
        flexWrap='wrap'
        justifyContent='center'
        gap={2}
      >
        {state.image && (
          <ImageWithTitle
            src={state.image.url || ''}
            alt={state.image.name}
            title={state.image.name}
          />
        )}
      </Box>
      {state.image && (
        <Box display='flex' flexDirection='row' gap={4} alignItems='center'>
          <Box
            display='flex'
            flexDirection='row'
            gap={2}
            justifyContent='center'
          >
            <Typography
              variant='body1'
              sx={{ display: 'flex', alignItems: 'center' }}
            >
              Result Language:
            </Typography>
            <Select
              value={state.selectedLanguage}
              onChange={(e) =>
                updateState({
                  selectedLanguage: e.target.value as Language,
                })
              }
            >
              <MenuItem value={Language.LANGUAGE_EN_US}>English</MenuItem>
              <MenuItem value={Language.LANGUAGE_KO_KR}>Korean</MenuItem>
              <MenuItem value={Language.LANGUAGE_JA_JP}>Japanese</MenuItem>
            </Select>
            <Button
              onClick={toImage}
              variant='contained'
              disabled={state.isLoading}
            >
              To Image
            </Button>
          </Box>
        </Box>
      )}
      {state.isLoading ? (
        <Box textAlign='center' sx={{ mt: 4 }}>
          <CircularProgress />
          <Typography variant='body1' sx={{ mt: 1 }}>
            Loading image...
          </Typography>
        </Box>
      ) : (
        state.translatedImage && (
          <img
            src={state.translatedImage}
            alt='Translated'
            style={{ objectFit: 'contain' }}
          />
        )
      )}
    </Stack>
  );
};

export default ImageToImage;
