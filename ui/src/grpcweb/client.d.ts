import { visionex } from './generated/grpc';
export declare const VISIONEX_TOKEN_KEY = "visionex-token";
export declare const translateToMarkdown: (image: Uint8Array, targetLanguage: visionex.grpc.Language, model: visionex.grpc.Model) => Promise<string>;
export declare const translateToImage: (image: Uint8Array, targetLanguage: visionex.grpc.Language) => Promise<string>;
export declare const translateTextFromImage: (image: Uint8Array, targetLanguage: visionex.grpc.Language) => Promise<{
    uriImage: string;
    sentences: ReturnType<typeof visionex.grpc.Sentence.prototype.toObject>[];
}>;
export declare const signInToVisionEx: (idToken: string) => Promise<void>;
