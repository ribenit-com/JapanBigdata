#!/usr/bin/env python3
import subprocess
import time
import psutil
import os
import socket
from selenium import webdriver
from selenium.webdriver.chrome.service import Service
from webdriver_manager.chrome import ChromeDriverManager

def check_port_available(port):
    """检查端口是否可用"""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    result = sock.connect_ex(('127.0.0.1', port))
    sock.close()
    return result == 0

def is_chrome_running_with_debug_port():
    """检查是否有带调试端口的Chrome正在运行"""
    print("检查Chrome进程...")
    for proc in psutil.process_iter(['pid', 'name', 'cmdline']):
        try:
            if proc.info['name'] == 'chrome.exe':
                cmdline = proc.info['cmdline']
                if cmdline:
                    print(f"找到Chrome进程: {proc.info['pid']}")
                    print(f"命令行: {' '.join(cmdline)}")
                if cmdline and '--remote-debugging-port=9222' in cmdline:
                    print("找到调试端口Chrome进程")
                    return True
        except (psutil.NoSuchProcess, psutil.AccessDenied) as e:
            print(f"检查进程时出错: {e}")
    return False

def kill_existing_chrome():
    """结束所有Chrome进程"""
    print("尝试结束现有Chrome进程...")
    for proc in psutil.process_iter(['pid', 'name']):
        try:
            if proc.info['name'] == 'chrome.exe':
                proc.kill()
                print(f"结束进程: {proc.info['pid']}")
        except (psutil.NoSuchProcess, psutil.AccessDenied) as e:
            print(f"结束进程时出错: {e}")
    time.sleep(2)

def get_chrome_path():
    """获取Chrome路径"""
    possible_paths = [
        r"C:\Users\Administrator\AppData\Local\Google\Chrome\Application\chrome.exe",
        r"C:\Users\Administrator\AppData\Local\Google\Chrome\Bin\chrome.exe",
        r"C:\Program Files\Google\Chrome\Application\chrome.exe",
        r"C:\Program Files (x86)\Google\Chrome\Application\chrome.exe"
    ]
    
    for path in possible_paths:
        if os.path.exists(path):
            print(f"找到Chrome路径: {path}")
            return path
    raise Exception("找不到Chrome浏览器")

def test_chrome_connection():
    print("开始测试Chrome连接...")
    
    try:
        # 1. 获取Chrome路径
        chrome_path = get_chrome_path()
        
        # 2. 结束现有Chrome进程
        kill_existing_chrome()
        
        # 3. 启动新的Chrome实例
        print("启动Chrome...")
        subprocess.Popen([
            chrome_path,
            "--remote-debugging-port=9222",
            "--user-data-dir=D:\\selenium",
            "--no-first-run",
            "--no-default-browser-check"
        ])
        
        # 4. 等待Chrome启动
        print("等待Chrome启动...")
        max_retries = 30
        for i in range(max_retries):
            if check_port_available(9222):
                print("Chrome已启动，端口9222可访问")
                break
            print(f"等待Chrome启动... ({i+1}/{max_retries})")
            time.sleep(1)
        else:
            raise Exception("Chrome启动超时")
        
        # 5. 连接到Chrome
        print("设置Selenium选项...")
        options = webdriver.ChromeOptions()
        options.add_experimental_option("debuggerAddress", "127.0.0.1:9222")
        service = Service(ChromeDriverManager().install())
        
        print("尝试连接到Chrome...")
        driver = webdriver.Chrome(service=service, options=options)
        print("成功连接到Chrome!")
        
        # 6. 测试连接
        url = driver.current_url
        print(f"当前页面: {url}")
        
        driver.quit()
        print("测试完成!")
        
    except Exception as e:
        print(f"错误: {e}")
        raise

if __name__ == "__main__":
    test_chrome_connection() 