import auth from '../auth';
import { visionex } from './generated/grpc';
const { TranslateToMarkdownRequest, TranslateToImageRequest, TranslateTextFromImageRequest, VisionExClient, SignInRequest, } = visionex.grpc;
export const VISIONEX_TOKEN_KEY = 'visionex-token';
const createAuthMetadata = async () => {
    const user = auth.currentUser;
    if (!user) {
        throw new Error('User not authenticated');
    }
    const token = await user.getIdToken();
    return {
        authorization: `Bearer ${token}`,
    };
};
const client = new VisionExClient(import.meta.env.VITE_API_ENDPOINT, {} /* =credentials */);
// TODO(#7518): Revisit client methods to throw errors instead of using Promise.
export const translateToMarkdown = async (image, targetLanguage, model) => {
    const response = await client.TranslateToMarkdown(TranslateToMarkdownRequest.fromObject({ image, targetLanguage, model }), await createAuthMetadata());
    const { markdown } = response.toObject();
    if (!markdown) {
        return Promise.reject(new Error('Failed to translate image to markdown'));
    }
    return markdown;
};
export const translateToImage = async (image, targetLanguage) => {
    const response = await client.TranslateToImage(TranslateToImageRequest.fromObject({ image, targetLanguage }), await createAuthMetadata());
    const { uriImage } = response.toObject();
    if (!uriImage) {
        return Promise.reject(new Error('Failed to translate markdown to image'));
    }
    return uriImage;
};
export const translateTextFromImage = async (image, targetLanguage) => {
    const response = await client.TranslateTextFromImage(TranslateTextFromImageRequest.fromObject({ image, targetLanguage }), await createAuthMetadata());
    const { uriImage, sentences } = response.toObject();
    if (!uriImage) {
        return Promise.reject(new Error('Failed to translate text from image'));
    }
    if (!sentences) {
        return Promise.reject(new Error('Failed to translate text from image'));
    }
    return { uriImage, sentences };
};
// TODO(#7518): Make SignIn authentication less.
export const signInToVisionEx = async (idToken) => {
    const response = await client.SignIn(SignInRequest.fromObject({ googleOpenIdToken: idToken }), await createAuthMetadata());
    const { token } = response.toObject();
    if (!token) {
        return Promise.reject(new Error('Failed to sign in to VisionEx'));
    }
    localStorage.setItem(VISIONEX_TOKEN_KEY, token);
    return undefined;
};
