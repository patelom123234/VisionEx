package com.visionex.clients;

import com.visionex.grpc.Language;
import java.awt.Font;
import java.io.InputStream;

public class FontProvider {
    private final FontsByFace englishFonts;
    private final FontsByFace japaneseFonts;
    private final FontsByFace koreanFonts;

    public FontProvider() throws Exception {
        this.englishFonts = loadFonts("/fonts/English");
        this.japaneseFonts = loadFonts("/fonts/Japanese");
        this.koreanFonts = loadFonts("/fonts/Korean");
    }

    public FontsByFace getFontByLanguage(Language language) {
        return switch (language) {
            case LANGUAGE_JA_JP -> japaneseFonts;
            case LANGUAGE_KO_KR -> koreanFonts;
            case LANGUAGE_EN_US -> englishFonts;
            default -> englishFonts;
        };
    }

    private FontsByFace loadFonts(String dir) throws Exception {
        Font regular = loadFontResource(dir + "/SansSerif-Regular.ttf");
        Font semiBold = loadFontResource(dir + "/SansSerif-SemiBold.ttf");
        Font bold = loadFontResource(dir + "/SansSerif-Bold.ttf");
        return new FontsByFace(regular, semiBold, bold);
    }

    private Font loadFontResource(String path) throws Exception {
        try (InputStream stream = FontProvider.class.getResourceAsStream(path)) {
            if (stream == null) {
                throw new IllegalStateException("font resource not found: " + path);
            }
            return Font.createFont(Font.TRUETYPE_FONT, stream);
        }
    }
}

