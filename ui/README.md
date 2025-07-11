# VisionEx-UI (Visual Exploration and Global Assistance UI)

This repository contains the UI project for VisionEx, which allows you to easily check the image translation process.

## Project Requirements Document

For more detailed information about the project, please refer to the [PRD](https://docs.google.com/document/d/1r2pq5nNafixDPQeEWsJDAlfcM9MwoovR8kxDNkAabL8/edit?usp=sharing).

## Running the UI Locally

To run this interface in your local development environment, use the following command:

```shell
# Install dependencies
npm install

# Start development server
npm run dev
```

## Building for Production

To build the UI for production:

```shell
npm run build
```

## Regenerate the proto file

To regenerate the proto file, use the following command from the project root:

```shell
make proto
```

## Development

This UI is built with:
- React 18
- TypeScript
- Vite
- gRPC-Web for communication with the backend

The UI provides interfaces for:
- Image-to-Image translation
- Image-to-Text translation  
- Image-to-Markdown conversion
- User authentication via Google Sign-In
