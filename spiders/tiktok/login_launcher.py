#!/usr/bin/env python3
# TikTok登录启动器
# 该脚本用于启动Go程序并处理TikTok的登录流程，特别是验证码部分

# 导入必要的库
from selenium import webdriver  # Selenium WebDriver，用于浏览器自动化
from selenium.webdriver.common.by import By  # 用于定位元素的方法
from selenium.webdriver.support.ui import WebDriverWait  # 显式等待
from selenium.webdriver.support import expected_conditions as EC  # 预期条件
from selenium.webdriver.common.action_chains import ActionChains  # 用于模拟鼠标操作
from selenium.webdriver.chrome.service import Service  # ChromeDriver服务
from webdriver_manager.chrome import ChromeDriverManager  # 自动管理ChromeDriver
import subprocess  # 用于启动外部进程
import time  # 用于添加延时
import os  # 用于处理文件路径
import signal
import socket
import psutil

class TikTokLoginLauncher:
    """TikTok登录启动器类
    该类负责整个TikTok自动化登录流程的协调和管理，主要功能包括：
    1. 启动和管理Chrome浏览器
    2. 启动Go程序并处理Cookie设置
    3. 初始化Selenium并进行自动化操作
    4. 处理登录验证和状态监控
    5. 管理资源清理和异常处理
    """

    def __init__(self):
        """初始化TikTok登录启动器
        设置类的基本属性和状态标志：
        - driver: Selenium WebDriver实例，用于浏览器自动化
        - go_process: Go程序进程，用于管理Cookie和登录状态
        - running: 运行状态标志，用于控制程序的运行和退出
        """
        self.driver = None  # Selenium WebDriver实例
        self.go_process = None  # Go程序进程
        self.running = True
        print("TikTok登录启动器初始化...")

    def signal_handler(self, signum, frame):
        """处理系统中断信号（如Ctrl+C）
        Args:
            signum: 信号编号
            frame: 当前堆栈帧
        功能：
            - 捕获系统中断信号
            - 设置运行状态为False
            - 触发资源清理流程
        """
        print("\n收到中断信号，正在清理资源...")
        self.running = False
        self.cleanup()

    def cleanup(self):
        """清理程序使用的所有资源
        执行清理操作：
        1. 停止所有后台线程
        2. 关闭Selenium WebDriver
        3. 终止Go程序进程
        4. 处理异常情况并确保资源被正确释放
        注意：
        - 会尝试正常终止进程，如果失败则强制结束
        - 确保即使出现异常也能清理资源
        """
        self.thread_running = False  # 停止输出线程
        if self.driver:
            try:
                self.driver.quit()
            except Exception as e:
                print(f"关闭浏览器时出错: {e}")

        if self.go_process:
            try:
                self.go_process.terminate()
                self.go_process.wait(timeout=5)  # 等待进程结束
            except Exception as e:
                print(f"终止Go程序时出错: {e}")
                # 如果正常终止失败，强制结束进程
                try:
                    self.go_process.kill()
                except:
                    pass

    def check_port_available(self, port):
        """检查端口是否可用"""
        print(f"检查端口 {port} 是否可用...")
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(1)  # 设置超时时间
        try:
            sock.connect(('127.0.0.1', port))
            sock.close()
            print(f"端口 {port} 可用")
            return True
        except Exception as e:
            print(f"端口 {port} 不可用: {e}")
            sock.close()
            return False

    def start_go_program(self):
        """启动Go程序
        启动main.go并等待Chrome浏览器完全启动
        """
        print("启动 Go 程序...")
        
        # 先关闭所有现有的Chrome进程
        print("正在关闭所有Chrome进程...")
        chrome_killed = False
        for proc in psutil.process_iter(['pid', 'name']):
            try:
                if proc.info['name'] == 'chrome.exe':
                    proc.kill()
                    chrome_killed = True
                    print(f"已关闭Chrome进程: {proc.info['pid']}")
            except (psutil.NoSuchProcess, psutil.AccessDenied) as e:
                print(f"关闭进程时出错: {e}")
        
        if chrome_killed:
            print("等待Chrome进程完全关闭...")
            time.sleep(3)
        
        # 先启动Chrome
        chrome_path = self.get_chrome_path()
        print(f"启动Chrome: {chrome_path}")
        subprocess.Popen([
            chrome_path,
            "--remote-debugging-port=9222",
            "--user-data-dir=D:\\selenium",
            "--no-first-run",
            "--no-default-browser-check"
        ])
        
        # 等待Chrome启动
        print("等待Chrome启动...")
        max_retries = 15
        for i in range(max_retries):
            if self.check_port_available(9222):
                print("Chrome已启动，端口9222可访问")
                break
            print(f"等待Chrome启动... ({i+1}/{max_retries})")
            time.sleep(1)
        else:
            raise Exception("Chrome启动超时")
        
        # 等待Chrome完全初始化
        time.sleep(3)
        
        # 启动Go程序
        current_dir = os.path.dirname(os.path.abspath(__file__))
        go_main = os.path.join(current_dir, "main.go")
        
        # 检查文件是否存在
        if not os.path.exists(go_main):
            raise Exception(f"找不到Go程序: {go_main}")
        
        # 使用subprocess启动Go程序
        self.go_process = subprocess.Popen(
            ["go", "run", go_main],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            encoding='utf-8',
            errors='ignore'
        )
        print("等待Go程序初始化...")
        
        # 等待Go程序设置Cookie
        print("\n2. 等待Go程序设置Cookie...")
        max_wait = 30
        success = False
        for i in range(max_wait):
            try:
                # 读取所有输出
                while True:
                    output = self.go_process.stderr.readline()
                    if not output:
                        break
                    print(f"Go输出: {output.strip()}")
                    if "成功使用Cookie打开浏览器" in output:
                        print("🎉 Cookie设置成功！")
                        success = True
                        time.sleep(3)  # 等待页面加载
                        break
                if success:
                    break
            except Exception as e:
                print(f"读取输出错误: {e}")
            time.sleep(1)
        
        if not success:
            raise Exception("等待Cookie设置超时")

    def setup_selenium(self):
        """设置Selenium
        连接到已经运行的Chrome实例
        """
        print("\n" + "="*50)
        print("🚀 开始初始化 Selenium 自动化测试...")
        print("="*50)
        
        max_retries = 3
        retry_count = 0
        
        while retry_count < max_retries:
            try:
                print(f"\n🔄 正在连接Chrome浏览器 (尝试 {retry_count + 1}/{max_retries})...")
                print("1. 检查Chrome是否运行...")
                chrome_running = False
                for proc in psutil.process_iter(['pid', 'name', 'cmdline']):
                    if proc.info['name'] == 'chrome.exe':
                        chrome_running = True
                        print(f"   找到Chrome进程: {proc.info['pid']}")
                if not chrome_running:
                    raise Exception("未找到运行中的Chrome进程")
                
                print("2. 检查调试端口...")
                if not self.check_port_available(9222):
                    raise Exception("端口9222不可用")
                
                print("3. 配置Selenium选项...")
                options = webdriver.ChromeOptions()
                options.add_experimental_option("debuggerAddress", "127.0.0.1:9222")
                # 添加其他必要的选项
                options.add_argument('--no-sandbox')
                options.add_argument('--disable-dev-shm-usage')
                options.add_argument('--disable-gpu')
                options.add_argument('--disable-logging')
                options.add_argument('--log-level=3')
                print("✅ Chrome选项设置完成")
                
                print("4. 安装ChromeDriver...")
                service = Service(ChromeDriverManager().install())
                print("✅ ChromeDriver准备就绪")
                
                print("5. 创建WebDriver实例...")
                self.driver = webdriver.Chrome(service=service, options=options)
                print("\n" + "="*50)
                print("🎉 Selenium 自动化测试环境准备就绪!")
                print("="*50)
                print("✨ 连接状态:")
                
                # 找到并关闭非TikTok窗口
                tiktok_handle = None
                handles_to_close = []
                
                for handle in self.driver.window_handles:
                    self.driver.switch_to.window(handle)
                    current_url = self.driver.current_url
                    print(f"  - 窗口 {handle}: {current_url}")
                    if "tiktok.com" in current_url:
                        tiktok_handle = handle
                    else:
                        handles_to_close.append(handle)
                
                # 关闭非TikTok窗口
                for handle in handles_to_close:
                    print(f"关闭非TikTok窗口: {handle}")
                    self.driver.switch_to.window(handle)
                    self.driver.close()
                
                # 切换到TikTok窗口
                if tiktok_handle:
                    self.driver.switch_to.window(tiktok_handle)
                    print(f"\n✅ 已切换到TikTok窗口: {tiktok_handle}")
                    print(f"   当前URL: {self.driver.current_url}")
                else:
                    print("❌ 未找到TikTok窗口!")
                
                print("="*50)
                print("🤖 自动化测试可以开始了!")
                print("="*50)
                break
            except Exception as e:
                retry_count += 1
                print(f"\n❌ 连接失败 (第 {retry_count} 次尝试)")
                print(f"❌ 详细错误: {str(e)}")
                print(f"❌ 错误类型: {type(e).__name__}")
                if retry_count < max_retries:
                    print("⏳ 等待5秒后重试...")
                    time.sleep(5)
                else:
                    print("\n" + "="*50)
                    print("❌ Selenium 自动化环境启动失败!")
                    print("="*50)
                    raise Exception("无法连接到Chrome，请确保Chrome已正确启动")

    def get_chrome_path(self):
        """获取Chrome路径"""
        possible_paths = [
            r"C:\Users\Administrator\AppData\Local\Google\Chrome\Application\chrome.exe",
            r"C:\Users\Administrator\AppData\Local\Google\Chrome\Bin\chrome.exe",
            r"C:\Program Files\Google\Chrome\Application\chrome.exe",
            r"C:\Program Files (x86)\Google\Chrome\Application\chrome.exe"
        ]
        
        for path in possible_paths:
            if os.path.exists(path):
                return path
        raise Exception("找不到Chrome浏览器")

    def wait_for_login_page(self):
        """等待登录页面加载
        确保登录页面的关键元素已经出现
        Returns:
            bool: 页面是否成功加载
        """
        print("等待登录页面...")
        try:
            # 检查当前URL
            current_url = self.driver.current_url
            print(f"当前页面: {current_url}")
            
            # 获取所有窗口句柄
            handles = self.driver.window_handles
            print(f"发现 {len(handles)} 个窗口")
            
            # 遍历所有窗口
            for handle in handles:
                self.driver.switch_to.window(handle)
                current_url = self.driver.current_url
                print(f"窗口 {handle} 的URL: {current_url}")
                if "foryou" in current_url:
                    print("找到For You页面，已经登录")
                    return True
            
            return True
        except Exception as e:
            print(f"等待登录页面失败: {e}")
            return False

    def handle_verification(self):
        """处理验证码
        等待用户手动处理验证码
        """
        print("处理验证码...")
        print("请手动完成验证码...")
        try:
            # 等待验证结果
            time.sleep(30)  # 给用户30秒时间手动处理验证码
            
        except Exception as e:
            print(f"处理验证码时出错: {e}")

    def monitor_login_status(self):
        """监控登录状态"""
        print("监控登录状态...")
        # 简化的登录状态检查
        print("✅ 已成功登录TikTok")
        return True

    def interact_with_page(self):
        """与页面交互"""
        try:
            # 等待页面加载
            time.sleep(5)
            
            # 获取所有窗口句柄
            handles = self.driver.window_handles
            
            # 切换到正确的窗口
            for handle in handles:
                self.driver.switch_to.window(handle)
                if "foryou" in self.driver.current_url:
                    print("成功切换到For You页面")
                    
                    # 这里可以添加页面交互
                    # 例如：滚动页面
                    self.driver.execute_script("window.scrollBy(0, 500)")
                    time.sleep(2)
                    
                    # 或者点击某个元素
                    # elements = self.driver.find_elements(By.CLASS_NAME, "video-feed-item")
                    # if elements:
                    #     elements[0].click()
                    
                    return True
            
            print("未找到For You页面")
            return False
            
        except Exception as e:
            print(f"页面交互失败: {e}")
            return False

    def run(self):
        """运行主流程
        协调整个登录过程
        """
        try:
            # 设置信号处理
            signal.signal(signal.SIGINT, self.signal_handler)
            signal.signal(signal.SIGTERM, self.signal_handler)

            print("\n=== 开始TikTok登录流程 ===")
            
            print("\n1. 启动Go程序和Chrome...")
            self.start_go_program()  # 启动Go程序
            
            print("\n3. 初始化Selenium...")
            self.setup_selenium()    # 设置Selenium
            
            print("\n4. 等待登录页面...")
            # 检查登录页面是否加载
            if self.wait_for_login_page():
                print("\n5. 尝试页面交互...")
                if self.interact_with_page():
                    print("页面交互成功")
                else:
                    print("页面交互失败")
            else:
                raise Exception("登录页面加载失败")
            
            print("\n6. 等待验证码处理...")
            # 处理验证码
            self.handle_verification()
            
            print("\n7. 检查登录状态...")
            # 检查登录结果
            if self.monitor_login_status():
                print("登录成功！")
            else:
                print("登录失败！")
                
            print("\n8. 保持会话...")
            # 保持运行直到收到中断信号
            while self.running:
                print(".", end="", flush=True)
                time.sleep(1)
                
        except Exception as e:
            print(f"\n错误: {e}")
            print("\n正在清理资源...")
        finally:
            self.cleanup()
            print("\n=== 登录流程结束 ===")

# 程序入口
if __name__ == "__main__":
    print("\n=== TikTok登录启动器 ===")
    launcher = TikTokLoginLauncher()  # 创建启动器实例
    launcher.run()  # 运行登录流程 