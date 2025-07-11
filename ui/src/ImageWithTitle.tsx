import { Box, Typography } from '@mui/material';

interface ImageWithTitleProps {
  src: string;
  alt: string;
  title: string;
}

const ImageWithTitle = ({ src, alt, title }: ImageWithTitleProps) => {
  return (
    <Box textAlign='center' m={2}>
      <img
        src={src}
        alt={alt}
        style={{
          maxWidth: '200px',
          maxHeight: '200px',
          display: 'block',
          margin: 'auto',
        }}
      />
      <Typography variant='body1' sx={{ mt: 1 }}>
        {title}
      </Typography>
    </Box>
  );
};

export default ImageWithTitle;
