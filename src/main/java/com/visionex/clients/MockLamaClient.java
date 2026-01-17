package com.visionex.clients;

import java.awt.Graphics2D;
import java.awt.image.BufferedImage;

public class MockLamaClient implements LamaClient {
    @Override
    public BufferedImage createMaskImage(BufferedImage originImage, BufferedImage maskImage) {
        BufferedImage result = new BufferedImage(
                originImage.getWidth(),
                originImage.getHeight(),
                BufferedImage.TYPE_INT_ARGB
        );
        Graphics2D graphics = result.createGraphics();
        graphics.drawImage(originImage, 0, 0, null);
        graphics.dispose();
        return result;
    }
}

